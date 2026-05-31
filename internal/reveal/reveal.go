// Package reveal owns the bar's slide animation and click-through state. It
// talks to the OS only through the WindowOps seam, so the slide logic can be
// exercised with a fake instead of a real window. The hover machine that decides
// *when* to reveal or collapse still lives on App and drives this controller via
// SetExpanded/Expanded; moving that in is issue #3.
package reveal

import (
	"sync"
	"sync/atomic"
	"time"

	"claudepanel/internal/platform"
)

// WindowOps is the narrow set of OS window operations the reveal machine needs.
// The production adapter binds a single window handle and forwards to
// internal/platform; tests inject a fake to assert slide positions and exercise
// generation/cancellation without a real OS window. Methods that don't take a
// handle in the platform layer (cursor/predicates) are on the seam too so the
// fake controls every input regardless of the host OS the test runs on.
type WindowOps interface {
	WindowRect() (left, top, width, height int)
	MoveTo(x, y int)
	ClipTop(width, height, topClip int)
	Show()
	Hide()
	SetClickThrough(enabled bool)
	CursorPos() (x, y int)
	FullScreenActive() bool
	AutoHideSupported() bool
}

const (
	defaultSlideDuration = 200 * time.Millisecond
	defaultFrame         = 16 * time.Millisecond // ~60 fps
)

// platformOps is the production WindowOps: it binds the window handle and
// forwards each call to the package-level internal/platform window functions.
type platformOps struct{ hwnd uintptr }

func (p platformOps) WindowRect() (int, int, int, int) { return platform.GetWindowSize(p.hwnd) }
func (p platformOps) MoveTo(x, y int)                  { platform.MoveWindow(p.hwnd, x, y) }
func (p platformOps) ClipTop(w, h, t int)              { platform.SetWindowClipTop(p.hwnd, w, h, t) }
func (p platformOps) Show()                            { platform.ShowWindow(p.hwnd) }
func (p platformOps) Hide()                            { platform.HideWindow(p.hwnd) }
func (p platformOps) SetClickThrough(e bool)           { platform.SetClickThrough(p.hwnd, e) }
func (p platformOps) CursorPos() (int, int)            { return platform.GetCursorPos() }
func (p platformOps) FullScreenActive() bool           { return platform.IsFullScreenActive() }
func (p platformOps) AutoHideSupported() bool          { return platform.AutoHideSupported() }

func realTicker(d time.Duration) (<-chan time.Time, func()) {
	t := time.NewTicker(d)
	return t.C, t.Stop
}

// Controller owns the slide animation and click-through state behind WindowOps.
// It holds a geometry/mode snapshot pushed in via Configure (refreshed on dock /
// pin / click-through changes) rather than reaching back into App config.
type Controller struct {
	ops       WindowOps
	now       func() time.Time
	newTicker func(time.Duration) (<-chan time.Time, func())
	slide     time.Duration
	frame     time.Duration

	// onDone, when non-nil, is invoked as each animateY goroutine returns (for
	// any reason, including supersede). Test-only hook; nil in production.
	onDone func(gen uint64)

	mu               sync.Mutex
	configured       bool
	mon              platform.MonitorInfo
	barHeight        int
	pinned           bool
	userClickThrough bool
	expanded         bool

	// animGen is bumped on every SetExpanded; a running animateY exits once it
	// sees the bump, so a new slide cleanly supersedes an in-flight one.
	animGen atomic.Uint64
}

// New builds a production Controller bound to the given native window handle.
func New(hwnd uintptr) *Controller {
	return newWithOps(platformOps{hwnd: hwnd})
}

// newWithOps is the test seam: it injects the WindowOps (a fake) and wires the
// real clock/ticker + default durations, which in-package tests may override.
func newWithOps(ops WindowOps) *Controller {
	return &Controller{
		ops:       ops,
		now:       time.Now,
		newTicker: realTicker,
		slide:     defaultSlideDuration,
		frame:     defaultFrame,
	}
}

// snapshot is a consistent read of the controller's geometry/mode state, taken
// under the lock so the animation/click-through math sees a coherent picture.
type snapshot struct {
	mon              platform.MonitorInfo
	barHeight        int
	pinned           bool
	userClickThrough bool
	expanded         bool
}

func (c *Controller) snap() snapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	return snapshot{c.mon, c.barHeight, c.pinned, c.userClickThrough, c.expanded}
}

func widthOf(mon platform.MonitorInfo) int {
	if mon.PhysWidth != 0 {
		return mon.PhysWidth
	}
	return mon.Width
}

// onScreenY is the bar's resting top, below any chrome above it (e.g. the macOS
// menu bar via WorkTopOffset). offScreenY is computed from the monitor's TRUE
// top so the window fully clears the screen when collapsed.
func onScreenY(s snapshot) int  { return int(s.mon.Top) + s.mon.WorkTopOffset }
func offScreenY(s snapshot) int { return int(s.mon.Top) - s.barHeight }

// Configure refreshes the geometry/mode snapshot and re-applies click-through.
// Call it wherever the bar is (re)docked and on pin / click-through changes.
func (c *Controller) Configure(mon platform.MonitorInfo, barHeight int, pinned, clickThrough bool) {
	c.mu.Lock()
	c.mon = mon
	c.barHeight = barHeight
	c.pinned = pinned
	c.userClickThrough = clickThrough
	c.configured = true
	c.mu.Unlock()
	c.ApplyClickThrough()
}

// SetUserClickThrough updates the user click-through preference and re-applies it
// (used by the tray toggle, which changes nothing about geometry).
func (c *Controller) SetUserClickThrough(enabled bool) {
	c.mu.Lock()
	c.userClickThrough = enabled
	c.mu.Unlock()
	c.ApplyClickThrough()
}

// Init sets the initial visual state without animating. When starting collapsed
// it snaps the window above the screen edge and hides it so nothing flashes on
// launch. Call after Configure.
func (c *Controller) Init(expanded bool) {
	c.mu.Lock()
	c.expanded = expanded
	s := snapshot{c.mon, c.barHeight, c.pinned, c.userClickThrough, expanded}
	c.mu.Unlock()

	c.ApplyClickThrough()
	if !expanded {
		c.ops.MoveTo(int(s.mon.Left), offScreenY(s))
		// Full clip so even if a monitor sits above, the window can't spill
		// onto it before Hide takes effect.
		c.ops.ClipTop(widthOf(s.mon), s.barHeight, s.barHeight)
		c.ops.Hide()
	}
}

// Expanded reports whether the bar is currently on-screen.
func (c *Controller) Expanded() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.expanded
}

// Reveal slides the bar on-screen (used by the single-instance re-launch path).
func (c *Controller) Reveal() { c.SetExpanded(true) }

// SetExpanded transitions the bar on/off screen by sliding the OS window itself
// (so the dark window background travels with the bar, leaving no leftover
// frame). It's a no-op if already in the target state or not yet configured.
// Every call supersedes any in-flight slide.
func (c *Controller) SetExpanded(expanded bool) {
	c.mu.Lock()
	if !c.configured || c.expanded == expanded {
		c.mu.Unlock()
		return
	}
	c.expanded = expanded
	s := snapshot{c.mon, c.barHeight, c.pinned, c.userClickThrough, expanded}
	c.mu.Unlock()

	c.ApplyClickThrough()

	target := onScreenY(s)
	if !expanded {
		target = offScreenY(s)
	}
	gen := c.animGen.Add(1)
	if expanded {
		c.ops.Show() // reveal the off-screen window so MoveTo can slide it in
	}
	go c.animateY(s, target, gen, !expanded)
}

// ApplyClickThrough sets the window's click-through from the user preference OR,
// where auto-hide is wired up, the "invisible collapsed" state — so a hidden bar
// can't eat clicks. On platforms without auto-hide this reduces to the user
// preference alone.
func (c *Controller) ApplyClickThrough() {
	s := c.snap()
	autoHide := c.ops.AutoHideSupported() && !s.pinned && !s.expanded
	c.ops.SetClickThrough(s.userClickThrough || autoHide)
}

// animateY slides the window's top edge to targetY over c.slide with an ease-out
// cubic, repositioning the top clip each frame so the portion above mon.Top stays
// masked (multi-monitor spill). If hideAfter, the window is hidden once it
// reaches the off-screen target. A newer SetExpanded bumps animGen; this loop
// sees the bump and exits without touching the window further.
func (c *Controller) animateY(s snapshot, targetY int, gen uint64, hideAfter bool) {
	if c.onDone != nil {
		defer c.onDone(gen)
	}
	x := int(s.mon.Left)
	monTop := int(s.mon.Top)
	width := widthOf(s.mon)
	barH := s.barHeight

	_, startY, _, _ := c.ops.WindowRect()
	if startY == targetY {
		if hideAfter {
			c.ops.Hide()
		}
		return
	}
	start := c.now()
	tickC, stop := c.newTicker(c.frame)
	defer stop()

	// Once any pixel has crossed above mon.Top, clip one extra pixel to absorb
	// DPI/rounding slop that would otherwise leave a row on the monitor above.
	clipFor := func(y int) int {
		top := monTop - y
		if top > 0 {
			top++
		}
		return top
	}

	for range tickC {
		if c.animGen.Load() != gen {
			return // superseded by a newer slide
		}
		elapsed := c.now().Sub(start)
		if elapsed >= c.slide {
			c.ops.MoveTo(x, targetY)
			c.ops.ClipTop(width, barH, clipFor(targetY))
			if hideAfter {
				c.ops.Hide()
			}
			return
		}
		t := float64(elapsed) / float64(c.slide)
		t = 1 - (1-t)*(1-t)*(1-t) // ease-out cubic
		y := startY + int(float64(targetY-startY)*t)
		c.ops.MoveTo(x, y)
		c.ops.ClipTop(width, barH, clipFor(y))
	}
}
