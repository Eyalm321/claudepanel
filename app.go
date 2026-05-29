package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"claudepanel/internal/audio"
	"claudepanel/internal/claude"
	"claudepanel/internal/config"
	"claudepanel/internal/platform"
	"claudepanel/internal/radio"
	"claudepanel/internal/station"
	"claudepanel/internal/terminal"
	"claudepanel/internal/tray"

	"context"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// radioResolverAdapter bridges *radio.Resolver to the audio.StreamResolver
// interface (different ResolvedTrack types keep the audio layer independent of
// the youtube-backed radio package).
type radioResolverAdapter struct{ r *radio.Resolver }

func (a radioResolverAdapter) Resolve(ctx context.Context, videoID string, forceRefresh bool) (audio.ResolvedTrack, error) {
	t, err := a.r.Resolve(ctx, videoID, forceRefresh)
	return audio.ResolvedTrack{URL: t.URL, IsLive: t.IsLive}, err
}

// Version is set via -ldflags "-X main.Version=x.y.z" at build time.
var Version = "dev"

// Grace period after the cursor leaves the bar before we collapse it. Lets
// the user briefly overshoot and come back without the bar snapping away.
const collapseDelay = 200 * time.Millisecond

type App struct {
	app       *application.App
	window    *application.WebviewWindow
	cfg       *config.Config
	monitors  []platform.MonitorInfo
	hwnd      uintptr
	trayMgr   *tray.Manager
	radio     *radio.Resolver
	audioCtrl *audio.Controller
	station   *station.StationPlayer

	// settingsWindow is the reusable popup editor. Created lazily on first use
	// and hidden (not destroyed) on close, so reopening is cheap.
	settingsWindow *application.WebviewWindow

	// hover-watcher state
	editorOpen  bool
	barExpanded bool      // true = window on screen at mon.Top; false = above the top edge / hidden
	leftBarAt   time.Time // first tick the cursor was off the bar — zero while it's on
	animGen     uint64    // bumped on each new slide; in-flight animations exit if superseded

	domReadyOnce bool // guards against WindowRuntimeReady firing twice on Windows WebView2
}

// Slide-animation duration. Keep in sync with collapseDelay's UX feel.
const slideDuration = 200 * time.Millisecond

func NewApp() *App {
	// Redirect log output to %APPDATA%\ClaudePanel\debug.log for crash diagnosis.
	logPath := filepath.Join(config.AppDataDir(), "debug.log")
	_ = os.MkdirAll(config.AppDataDir(), 0755)
	if f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644); err == nil {
		log.SetOutput(f)
		log.SetFlags(log.Ldate | log.Ltime)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Printf("config load error: %v — using defaults", err)
		def := config.Defaults()
		cfg = &def
	}
	// Always launch pinned regardless of last-session state — the user prefers
	// to start with the bar docked, even if they collapsed it last time.
	cfg.Pinned = true
	cfg.AppBarMode = true
	res := radio.New()
	app := &App{cfg: cfg, radio: res}

	ctrl, err := audio.NewController(radioResolverAdapter{res}, func(ev audio.Event) {
		// Route every controller event through the station player, which
		// auto-advances/loops and then forwards (enriched) to the frontend.
		if app.station != nil {
			app.station.OnAudioEvent(ev)
		} else if app.app != nil {
			app.app.Event.Emit("radio:state", ev)
		}
	})
	if err != nil {
		log.Printf("[audio] Failed to initialize native audio controller: %v", err)
	} else {
		app.audioCtrl = ctrl
		app.station = station.New(ctrl, res, func(ev audio.Event) {
			if app.app != nil {
				app.app.Event.Emit("radio:state", ev)
			}
		})
		app.station.SetStations(cfg.Stations)
	}

	return app
}

func (a *App) startup(app *application.App, window *application.WebviewWindow) {
	a.app = app
	a.window = window
}

func (a *App) domReady(app *application.App, window *application.WebviewWindow) {
	// WindowRuntimeReady can fire more than once on Windows WebView2 (initial
	// about:blank + the real asset load, or any in-app navigation). Without this
	// guard each fire would build a second tray icon and spawn a second
	// hover-watcher goroutine.
	if a.domReadyOnce {
		return
	}
	a.domReadyOnce = true

	time.Sleep(300 * time.Millisecond)

	hwnd := uintptr(window.NativeWindow())
	a.hwnd = hwnd
	platform.ApplyBarStyles(hwnd)

	a.monitors = platform.GetMonitors()
	if a.cfg.Monitor >= len(a.monitors) {
		a.cfg.Monitor = 0
	}

	if a.hwnd != 0 && len(a.monitors) > 0 {
		platform.DockToMonitor(a.hwnd, a.monitors[a.cfg.Monitor], a.cfg.BarHeight, a.cfg.AppBarMode)
		if a.cfg.AppBarMode && a.cfg.Pinned {
			go func() {
				if err := platform.PushdownEnable(a.monitors[a.cfg.Monitor], a.cfg.BarHeight); err != nil {
					log.Printf("[pushdown] Enable failed: %v", err)
				}
			}()
		}
		platform.SetOpacity(a.hwnd, a.cfg.Opacity)
		if a.cfg.Pinned {
			a.barExpanded = true
		} else {
			a.barExpanded = a.cursorOverBar()
		}
		a.applyClickThrough()
		// If the bar starts collapsed, snap the window above the screen edge
		// AND hide it so nothing flashes on launch.
		if !a.barExpanded {
			mon := a.monitors[a.cfg.Monitor]
			width := mon.PhysWidth
			if width == 0 {
				width = mon.Width
			}
			platform.MoveWindow(a.hwnd, int(mon.Left), int(mon.Top)-a.cfg.BarHeight)
			// Full clip so even if a monitor sits above, the window can't
			// spill onto it before SW_HIDE takes effect.
			platform.SetWindowClipTop(a.hwnd, width, a.cfg.BarHeight, a.cfg.BarHeight)
			platform.HideWindow(a.hwnd)
		}
	}

	a.runTray()
	go a.runHoverWatcher()
}

// runHoverWatcher polls the cursor position and drives the bar's expand/collapse
// when unpinned. WebView2's mouseleave is unreliable on small windows (it never
// fires when the cursor exits at the bottom edge), so we don't trust JS events
// for the hide trigger.
func (a *App) runHoverWatcher() {
	ticker := time.NewTicker(80 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-a.app.Context().Done():
			return
		case <-ticker.C:
			a.checkHover()
		}
	}
}

// cursorOverBar reports whether the OS cursor is currently inside the bar's
// monitor strip (top BarHeight pixels of the bar's monitor).
func (a *App) cursorOverBar() bool {
	if a.hwnd == 0 || len(a.monitors) == 0 {
		return false
	}
	mon := a.monitors[a.cfg.Monitor]
	cx, cy := platform.GetCursorPos()
	if cx < 0 && cy < 0 {
		return false // platform stub (macOS/Linux)
	}
	width := mon.PhysWidth
	if width == 0 {
		width = mon.Width
	}
	// The hover hit box runs from the monitor's true top edge down to the
	// bar's bottom edge. On macOS the bar rests at Top+WorkTopOffset (below
	// the menu bar); we still want cursor presence in the menu-bar slice
	// above the bar to count as "on the bar" so the user can reveal it from
	// the screen edge. On Windows/Linux WorkTopOffset is 0 and this collapses
	// to the original [Top, Top+BarHeight] check.
	return cx >= int(mon.Left) && cx < int(mon.Left)+width &&
		cy >= int(mon.Top) && cy < int(mon.Top)+mon.WorkTopOffset+a.cfg.BarHeight
}

func (a *App) checkHover() {
	if a.hwnd == 0 || a.editorOpen {
		return
	}
	// Fullscreen takes precedence over pin/hover: while any frontmost-app
	// main window is in native macOS fullscreen, force-collapse the bar (the
	// tray icon stays). On Windows/Linux IsFullScreenActive is a stub and
	// always returns false, so this is a no-op there.
	if platform.IsFullScreenActive() {
		if a.barExpanded {
			a.setBarExpanded(false)
		}
		return
	}
	if a.cfg.Pinned {
		// Re-expand on exit-fullscreen if we suppressed earlier.
		if !a.barExpanded {
			a.setBarExpanded(true)
		}
		return
	}
	if a.cursorOverBar() {
		a.leftBarAt = time.Time{}
		a.setBarExpanded(true)
		return
	}
	// Cursor off the bar — start grace timer on the first off-tick; only
	// collapse once the cursor has been gone for collapseDelay.
	if a.leftBarAt.IsZero() {
		a.leftBarAt = time.Now()
		return
	}
	if a.barExpanded && time.Since(a.leftBarAt) >= collapseDelay {
		a.setBarExpanded(false)
	}
}

// setBarExpanded transitions the visual state by sliding the OS window
// up or down. We move the window itself rather than animating a CSS
// transform inside the bar so the dark window background travels with
// the bar — no leftover frame is left behind when the bar slides out.
func (a *App) setBarExpanded(expanded bool) {
	if a.barExpanded == expanded || a.hwnd == 0 || len(a.monitors) == 0 {
		return
	}
	a.barExpanded = expanded
	a.applyClickThrough()

	mon := a.monitors[a.cfg.Monitor]
	// Resting top accounts for any chrome above the bar (macOS menu bar).
	// The off-screen target is computed from the monitor's TRUE top so the
	// window clears the screen, not just the bar's resting slot.
	onScreenY := int(mon.Top) + mon.WorkTopOffset
	offScreenY := int(mon.Top) - a.cfg.BarHeight
	target := onScreenY
	if !expanded {
		target = offScreenY
	}

	a.animGen++
	if expanded {
		// Reveal the (off-screen) window so SetWindowPos can move it.
		platform.ShowWindow(a.hwnd)
	}
	go a.animateY(mon, target, a.animGen, !expanded)
}

// animateY slides the window's top edge to targetY over slideDuration with
// an ease-out cubic. At every frame we also reposition the SetWindowRgn clip
// so the portion of the window above mon.Top is masked out — without this,
// users with a monitor stacked above would see the bar appear on that
// monitor as it slides up. If `hideAfter` is set, the window is SW_HIDE'd
// once it reaches the off-screen target — defence-in-depth so a
// partially-visible window on a multi-monitor edge case still becomes
// truly invisible.
//
// Cancellation: every new setBarExpanded bumps a.animGen. A running
// animation sees the bump and exits without touching the window further.
func (a *App) animateY(mon platform.MonitorInfo, targetY int, gen uint64, hideAfter bool) {
	x := int(mon.Left)
	monTop := int(mon.Top)
	width := mon.PhysWidth
	if width == 0 {
		width = mon.Width
	}
	barH := a.cfg.BarHeight

	_, startY, _, _ := platform.GetWindowSize(a.hwnd)
	if startY == targetY {
		if hideAfter {
			platform.HideWindow(a.hwnd)
		}
		return
	}
	start := time.Now()
	ticker := time.NewTicker(16 * time.Millisecond) // ~60 fps
	defer ticker.Stop()
	// Whenever any pixel of the window has crossed above mon.Top, we clip
	// one extra pixel to absorb DPI/rounding slop — without the +1, a single
	// row sometimes lingers on the monitor above before the window fully
	// disappears.
	clipFor := func(y int) int {
		top := monTop - y
		if top > 0 {
			top++
		}
		return top
	}

	for range ticker.C {
		if a.animGen != gen {
			return // superseded
		}
		elapsed := time.Since(start)
		if elapsed >= slideDuration {
			platform.MoveWindow(a.hwnd, x, targetY)
			platform.SetWindowClipTop(a.hwnd, width, barH, clipFor(targetY))
			if hideAfter {
				platform.HideWindow(a.hwnd)
			}
			return
		}
		t := float64(elapsed) / float64(slideDuration)
		t = 1 - (1-t)*(1-t)*(1-t) // ease-out cubic
		y := startY + int(float64(targetY-startY)*t)
		platform.MoveWindow(a.hwnd, x, y)
		platform.SetWindowClipTop(a.hwnd, width, barH, clipFor(y))
	}
}

// applyClickThrough sets the window's click-through state based on the
// combination of the user-configurable cfg.ClickThrough and (on platforms
// where auto-hide is wired up) whether the bar is currently in its
// "invisible collapsed" state. On macOS / Linux the slide animation is a
// no-op, so engaging click-through there would just leave a visible-but-
// unclickable bar — skip it.
func (a *App) applyClickThrough() {
	if a.hwnd == 0 {
		return
	}
	autoHide := platform.AutoHideSupported() && !a.cfg.Pinned && !a.barExpanded
	platform.SetClickThrough(a.hwnd, a.cfg.ClickThrough || autoHide)
}

// reveal surfaces the bar, called when the user launches a second instance (which
// exits immediately under single-instance mode). If the bar was auto-hidden /
// collapsed it slides back on screen; when pinned and already visible it's a
// no-op. Guarded on hwnd so it does nothing if the first instance is still
// mid-startup.
func (a *App) reveal() {
	if a.hwnd == 0 {
		return
	}
	a.setBarExpanded(true)
}

func (a *App) shutdown() {
	platform.PushdownDisable()
	if a.hwnd != 0 {
		platform.RemoveAppBar(a.hwnd)
	}
	if a.trayMgr != nil {
		a.trayMgr.Quit()
	}
}

func (a *App) runTray() {
	names := make([]string, len(a.cfg.Accounts))
	for i, acc := range a.cfg.Accounts {
		names[i] = acc.Name
	}
	a.trayMgr = tray.New()
	a.trayMgr.Build(
		a.app,
		a,
		trayIconBytes,
		Version,
		names,
		len(a.monitors),
		a.cfg.StartWithWindows,
		a.cfg.ActiveAccount,
		a.cfg.Monitor,
	)
}

// tray.Controller implementation callbacks

func (a *App) ToggleStartup() {
	a.cfg.StartWithWindows = !a.cfg.StartWithWindows
	exePath, _ := os.Executable()
	_ = config.SetStartOnLogin(a.cfg.StartWithWindows, exePath)
	_ = config.Save(a.cfg)
	if a.trayMgr != nil {
		a.trayMgr.SetStartup(a.cfg.StartWithWindows)
	}
}

func (a *App) ConfigureAccounts()  { a.openSettings("accounts") }
func (a *App) ConfigureTerminals() { a.openSettings("terminals") }
func (a *App) ConfigureStations()  { a.openSettings("stations") }

// openSettings shows the reusable settings popup focused on the given panel
// ("accounts", "terminals", or "stations"). The window is its own frameless
// WebviewWindow (the bar itself is only BarHeight tall, with no room for a
// modal). It is created lazily and hidden — not destroyed — on close, so
// reopening preserves page state and is cheap.
//
// The target panel is delivered two ways for robustness: encoded in the URL on
// first creation (the page can't have registered an event listener yet), and
// re-sent via the "settings:show" event for every subsequent open / panel
// switch on the already-loaded page.
func (a *App) openSettings(panel string) {
	if a.app == nil {
		return
	}
	if a.settingsWindow == nil {
		a.settingsWindow = a.app.Window.NewWithOptions(application.WebviewWindowOptions{
			Name:             "settings",
			Title:            "Claude Panel",
			Width:            560,
			Height:           420,
			MinWidth:         420,
			MinHeight:        260,
			Frameless:        true,
			AlwaysOnTop:      true,
			DisableResize:    true,
			Hidden:           true,
			BackgroundColour: application.NewRGB(0x0B, 0x0C, 0x0E),
			URL:              "/settings.html?panel=" + panel,
		})
	}
	a.settingsWindow.Show()
	a.settingsWindow.Center()
	a.settingsWindow.Focus()
	a.app.Event.Emit("settings:show", panel)
}

func (a *App) Quit() {
	a.app.Quit()
}

// ── Wails-exported bindings ──────────────────────────────────────────────────

func (a *App) GetBarData() (*claude.BarData, error) {
	if len(a.cfg.Accounts) == 0 {
		return nil, fmt.Errorf("no accounts configured")
	}

	activeIdx := a.cfg.ActiveAccount
	if activeIdx >= len(a.cfg.Accounts) {
		activeIdx = 0
	}

	acc := a.cfg.Accounts[activeIdx]

	sc, _ := claude.ReadStatsCache(acc.Path)
	creds, _ := claude.ReadCredentials(acc.Path)
	sessions := claude.ReadSessions(acc.Path)
	notifs := claude.ReadNotifications(acc.Path)

	apiUsage := claude.ReadUsage(acc.Path)

	return claude.ComputeBarData(
		acc.Name,
		sc, creds, sessions, notifs,
		apiUsage,
	), nil
}

func (a *App) GetConfig() config.Config {
	return *a.cfg
}

func (a *App) SaveConfig(cfg config.Config) error {
	prevMonitor := a.cfg.Monitor
	prevAppBar := a.cfg.AppBarMode
	a.cfg = &cfg
	if err := config.Save(a.cfg); err != nil {
		return err
	}
	if a.station != nil {
		a.station.SetStations(cfg.Stations)
	}
	if a.hwnd != 0 {
		if cfg.Monitor != prevMonitor || cfg.AppBarMode != prevAppBar {
			if prevAppBar {
				platform.RemoveAppBar(a.hwnd)
			}
			a.monitors = platform.GetMonitors()
			if cfg.Monitor < len(a.monitors) {
				platform.DockToMonitor(a.hwnd, a.monitors[cfg.Monitor], cfg.BarHeight, cfg.AppBarMode)
			}
		}
		platform.SetOpacity(a.hwnd, cfg.Opacity)

		if cfg.AppBarMode && cfg.Pinned {
			go func() {
				if err := platform.PushdownEnable(a.monitors[cfg.Monitor], cfg.BarHeight); err != nil {
					log.Printf("[pushdown] Enable failed: %v", err)
				}
			}()
		} else {
			platform.PushdownDisable()
		}
	}
	a.app.Event.Emit("config:changed")
	return nil
}

func (a *App) GetMonitors() []platform.MonitorInfo {
	a.monitors = platform.GetMonitors()
	return a.monitors
}

func (a *App) SetActiveAccount(index int) error {
	if index < 0 || index >= len(a.cfg.Accounts) {
		return fmt.Errorf("account index %d out of range", index)
	}
	a.cfg.ActiveAccount = index
	if err := config.Save(a.cfg); err != nil {
		return err
	}
	if a.trayMgr != nil {
		a.trayMgr.SetAccountChecked(index)
	}
	a.app.Event.Emit("account:changed", index)
	return nil
}

func (a *App) SetMonitor(index int) error {
	a.monitors = platform.GetMonitors()
	if index < 0 || index >= len(a.monitors) {
		return fmt.Errorf("monitor index %d out of range", index)
	}
	if a.hwnd != 0 && a.cfg.AppBarMode {
		platform.RemoveAppBar(a.hwnd)
	}
	a.cfg.Monitor = index
	if err := config.Save(a.cfg); err != nil {
		return err
	}
	if a.hwnd != 0 {
		platform.DockToMonitor(a.hwnd, a.monitors[index], a.cfg.BarHeight, a.cfg.AppBarMode)
		platform.PushdownReconfigure(a.monitors[index], a.cfg.BarHeight)
	}
	if a.trayMgr != nil {
		a.trayMgr.SetMonitorChecked(index)
	}
	a.app.Event.Emit("monitor:changed", index)
	return nil
}

func (a *App) ToggleClickThrough() bool {
	a.cfg.ClickThrough = !a.cfg.ClickThrough
	a.applyClickThrough()
	_ = config.Save(a.cfg)
	return a.cfg.ClickThrough
}

func (a *App) SetOpacity(opacity float64) error {
	a.cfg.Opacity = opacity
	if a.hwnd != 0 {
		platform.SetOpacity(a.hwnd, opacity)
	}
	return config.Save(a.cfg)
}

func (a *App) GetVersion() string {
	return Version
}

// RadioPlayStation starts (or resumes) the configured station at index. The
// station player owns the queue, shuffle, auto-advance and looping; it drives
// the single-track audio controller one track at a time.
func (a *App) RadioPlayStation(index int) error {
	if a.station == nil {
		return fmt.Errorf("audio controller not initialized")
	}
	return a.station.Play(index)
}

// ParseStationItem classifies a single URL/ID into a StationItem for the
// stations editor (so URL parsing stays authoritative on the Go side).
func (a *App) ParseStationItem(input string) (config.StationItem, error) {
	return station.ParseItem(input)
}

// SetActiveStation persists which station is selected in the bar cycler. It
// does not start playback (use RadioPlayStation for that).
func (a *App) SetActiveStation(index int) error {
	if index < 0 || index >= len(a.cfg.Stations) {
		return fmt.Errorf("station index %d out of range", index)
	}
	a.cfg.ActiveStation = index
	return config.Save(a.cfg)
}

func (a *App) RadioPause() error {
	if a.station == nil {
		return fmt.Errorf("audio controller not initialized")
	}
	return a.station.Pause()
}

func (a *App) RadioSetVolume(v float64) error {
	if a.station == nil {
		return fmt.Errorf("audio controller not initialized")
	}
	// Persist the chosen volume so it survives restarts (replaces the prior
	// localStorage-only value).
	a.cfg.RadioVolume = v
	_ = config.Save(a.cfg)
	return a.station.SetVolume(v)
}

func (a *App) SetPinned(pinned bool) error {
	a.cfg.Pinned = pinned
	a.cfg.AppBarMode = pinned
	if err := config.Save(a.cfg); err != nil {
		return err
	}
	if a.hwnd != 0 && len(a.monitors) > 0 {
		platform.RemoveAppBar(a.hwnd)
		platform.DockToMonitor(a.hwnd, a.monitors[a.cfg.Monitor], a.cfg.BarHeight, pinned)
		if pinned {
			go func() {
				if err := platform.PushdownEnable(a.monitors[a.cfg.Monitor], a.cfg.BarHeight); err != nil {
					log.Printf("[pushdown] Enable failed: %v", err)
				}
			}()
		} else {
			platform.PushdownDisable()
		}
		a.leftBarAt = time.Time{}
		// Pinned ⇒ always expanded. Unpinned ⇒ initial state follows the
		// cursor (the user just clicked the pin icon, so the cursor is on the
		// bar; this avoids a flicker before the next polling tick).
		if pinned {
			a.setBarExpanded(true)
		} else {
			a.setBarExpanded(a.cursorOverBar())
		}
	}
	a.app.Event.Emit("pinned:changed", pinned)
	return nil
}

// SetEditorOpen forces the bar fully expanded while the inline accounts editor
// is shown (the editor is launched from the tray with the cursor off-bar, and
// must stay open until dismissed). On close, the hover-watcher re-evaluates
// based on the current cursor position.
func (a *App) SetEditorOpen(open bool) {
	a.editorOpen = open
	if open && !a.cfg.Pinned {
		a.setBarExpanded(true)
	}
	if !open {
		a.checkHover()
	}
}

// GetPushdownStats returns active diagnostics for macOS window pushdown.
func (a *App) GetPushdownStats() platform.PushdownStats {
	return platform.GetPushdownStats()
}

// OpenTerminal launches the configured launcher entry at index in a new,
// visible terminal window. The launcher program is resolved lazily on first
// use (no terminal detection happens in config.Defaults) and persisted.
func (a *App) OpenTerminal(index int) error {
	if index < 0 || index >= len(a.cfg.Terminals) {
		return fmt.Errorf("terminal index %d out of range", index)
	}
	if a.cfg.Launcher.Preset == "" {
		a.cfg.Launcher = terminal.DetectDefault()
		if err := config.Save(a.cfg); err != nil {
			log.Printf("[terminal] failed to persist detected launcher: %v", err)
		} else {
			log.Printf("[terminal] detected launcher preset %q", a.cfg.Launcher.Preset)
		}
	}
	entry := a.cfg.Terminals[index]
	if err := terminal.Launch(entry, a.cfg.Launcher); err != nil {
		log.Printf("[terminal] open %q via %s failed: %v", entry.Name, a.cfg.Launcher.Preset, err)
		return fmt.Errorf("failed to open terminal: %w", err)
	}
	log.Printf("[terminal] opened %q via %s", entry.Name, a.cfg.Launcher.Preset)
	return nil
}

// ListTerminalPresets returns the builtin terminal programs for this OS plus
// the custom escape hatch, for the bar editor's dropdown.
func (a *App) ListTerminalPresets() []terminal.PresetInfo {
	return terminal.Presets()
}

// DetectTerminal returns the auto-detected launcher for this machine without
// persisting it — used by the editor to preselect a sensible default.
func (a *App) DetectTerminal() config.LauncherConfig {
	return terminal.DetectDefault()
}

// PickDirectory opens a native folder-picker and returns the chosen absolute
// path, or "" if the user cancelled. The terminal editor uses this for the DIR
// field — a browser file input can't yield a real OS path in WebView2.
func (a *App) PickDirectory() (string, error) {
	dlg := a.app.Dialog.OpenFile().
		CanChooseDirectories(true).
		CanChooseFiles(false).
		CanCreateDirectories(true).
		SetTitle("Select a directory")
	if a.window != nil {
		dlg.AttachToWindow(a.window)
	}
	return dlg.PromptForSingleSelection()
}
