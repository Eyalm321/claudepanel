package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"claudepanel/internal/claude"
	"claudepanel/internal/config"
	"claudepanel/internal/platform"
	"claudepanel/internal/tray"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Version is set via -ldflags "-X main.Version=x.y.z" at build time.
var Version = "dev"

type App struct {
	ctx      context.Context
	cfg      *config.Config
	monitors []platform.MonitorInfo
	hwnd     uintptr
	trayMgr  *tray.Manager
}

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
	return &App{cfg: cfg}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) domReady(ctx context.Context) {
	time.Sleep(300 * time.Millisecond)

	hwnd, err := platform.FindWindowByPID()
	if err != nil {
		log.Printf("HWND lookup failed: %v", err)
	} else {
		a.hwnd = hwnd
		platform.ApplyBarStyles(hwnd)
	}

	a.monitors = platform.GetMonitors()
	if a.cfg.Monitor >= len(a.monitors) {
		a.cfg.Monitor = 0
	}

	if a.hwnd != 0 && len(a.monitors) > 0 {
		platform.DockToMonitor(a.hwnd, a.monitors[a.cfg.Monitor], a.cfg.BarHeight, a.cfg.AppBarMode)
		platform.SetOpacity(a.hwnd, a.cfg.Opacity)
		if a.cfg.ClickThrough {
			platform.SetClickThrough(a.hwnd, true)
		}
	}

	go a.runTray()
}

func (a *App) shutdown(ctx context.Context) {
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
	go a.handleTrayEvents()
	a.trayMgr.Run(trayIconBytes, Version, names, len(a.monitors))
}

func (a *App) handleTrayEvents() {
	for event := range a.trayMgr.Events() {
		switch event.Type {
		case tray.EventSetAccount:
			_ = a.SetActiveAccount(event.Index)
		case tray.EventSetMonitor:
			_ = a.SetMonitor(event.Index)
		case tray.EventToggleClickThrough:
			enabled := a.ToggleClickThrough()
			a.trayMgr.SetClickThrough(enabled)
		case tray.EventToggleStartup:
			a.cfg.StartWithWindows = !a.cfg.StartWithWindows
			exePath, _ := os.Executable()
			_ = config.SetStartOnLogin(a.cfg.StartWithWindows, exePath)
			_ = config.Save(a.cfg)
			a.trayMgr.SetStartup(a.cfg.StartWithWindows)
		case tray.EventManageAccounts:
			runtime.EventsEmit(a.ctx, "show:accounts-editor")
		case tray.EventQuit:
			a.trayMgr.Quit()
			runtime.Quit(a.ctx)
		}
	}
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
	}
	runtime.EventsEmit(a.ctx, "config:changed")
	return nil
}

func (a *App) GetMonitors() []platform.MonitorInfo {
	a.monitors = platform.GetMonitors()
	return a.monitors
}

// PlatformGOOS exposes runtime.GOOS to the frontend so platform-specific
// UI affordances (e.g. hiding AppBar mode on macOS) can branch on it.
func (a *App) PlatformGOOS() string {
	return goosString()
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
	runtime.EventsEmit(a.ctx, "account:changed", index)
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
	}
	if a.trayMgr != nil {
		a.trayMgr.SetMonitorChecked(index)
	}
	runtime.EventsEmit(a.ctx, "monitor:changed", index)
	return nil
}

func (a *App) ToggleClickThrough() bool {
	a.cfg.ClickThrough = !a.cfg.ClickThrough
	if a.hwnd != 0 {
		platform.SetClickThrough(a.hwnd, a.cfg.ClickThrough)
	}
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
