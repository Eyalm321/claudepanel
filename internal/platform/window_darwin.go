//go:build darwin

package platform

/*
#cgo darwin CFLAGS: -x objective-c -fobjc-arc
#cgo darwin LDFLAGS: -framework Cocoa
#import <Cocoa/Cocoa.h>

// Every NSWindow / NSApp / NSScreen mutation must happen on the main thread.
// Wails invokes OnDomReady / Wails-bound methods on a background goroutine, so
// raw cgo calls into AppKit from there get killed by AppKit's main-thread
// safety check (SIGTRAP on modern macOS). runOnMain bridges to the main queue
// — synchronously, since callers expect the work to be done by return time.
static inline void runOnMain(dispatch_block_t block) {
    if ([NSThread isMainThread]) {
        block();
    } else {
        dispatch_sync(dispatch_get_main_queue(), block);
    }
}

void platformApplyBarStyles(void* nsWindow) {
    runOnMain(^{
        NSWindow* w = (__bridge NSWindow*)nsWindow;
        if (!w) return;
        [w setStyleMask:NSWindowStyleMaskBorderless];
        [w setLevel:NSStatusWindowLevel];
        [w setCollectionBehavior:(NSWindowCollectionBehaviorCanJoinAllSpaces | NSWindowCollectionBehaviorStationary)];
        [w setHasShadow:NO];
        [w setMovable:NO];
    });
}

void platformSetOpacity(void* nsWindow, double opacity) {
    runOnMain(^{
        NSWindow* w = (__bridge NSWindow*)nsWindow;
        if (!w) return;
        double a = opacity;
        if (a < 0) a = 0;
        if (a > 1) a = 1;
        [w setAlphaValue:a];
    });
}

void platformSetClickThrough(void* nsWindow, int enabled) {
    runOnMain(^{
        NSWindow* w = (__bridge NSWindow*)nsWindow;
        if (!w) return;
        [w setIgnoresMouseEvents:(enabled ? YES : NO)];
    });
}

void platformDockToMonitor(void* nsWindow, int left, int top, int width, int height) {
    runOnMain(^{
        NSWindow* w = (__bridge NSWindow*)nsWindow;
        if (!w) return;
        // The system menu bar always renders above every window level, so we
        // position at the top of [NSScreen visibleFrame] (the area below the
        // menu bar) rather than at the screen's true top edge.
        NSScreen* main = [NSScreen mainScreen];
        if (!main) return;
        NSRect visible = [main visibleFrame];
        CGFloat y = visible.origin.y + visible.size.height - (CGFloat)height;
        NSRect frame = NSMakeRect((CGFloat)left, y, (CGFloat)width, (CGFloat)height);
        [w setFrame:frame display:YES];
        // Mirror the Windows DockToMonitor (SWP_SHOWWINDOW): explicitly order
        // the window in. The Wails options set Hidden:true so the framework
        // defers Show() until WindowDidBecomeKey, which an Accessory-policy
        // app never receives without user interaction — without this, the bar
        // is positioned correctly but stays invisible on first launch.
        [w orderFront:nil];
    });
}

void platformShowWindow(void* nsWindow) {
    runOnMain(^{
        NSWindow* w = (__bridge NSWindow*)nsWindow;
        if (!w) return;
        [w orderFront:nil];
    });
}

void platformHideWindow(void* nsWindow) {
    runOnMain(^{
        NSWindow* w = (__bridge NSWindow*)nsWindow;
        if (!w) return;
        [w orderOut:nil];
    });
}

void platformMoveWindow(void* nsWindow, int x, int y) {
    runOnMain(^{
        NSWindow* w = (__bridge NSWindow*)nsWindow;
        if (!w) return;
        NSScreen* main = [NSScreen mainScreen];
        if (!main) return;
        // Caller uses top-left-origin Y (matching Windows). Convert to
        // Cocoa's bottom-left-origin: flipY = primaryHeight - y - frameHeight.
        NSRect f = [w frame];
        CGFloat primaryH = main.frame.size.height;
        CGFloat cocoaY = primaryH - (CGFloat)y - f.size.height;
        [w setFrameOrigin:NSMakePoint((CGFloat)x, cocoaY)];
    });
}

// Cursor position in our top-left-origin convention (matching mon.Top/Left).
// NSEvent.mouseLocation returns Cocoa bottom-left coords on the primary
// screen; convert by flipping against the primary screen's height. Works
// from any app activation policy, including Accessory.
void platformGetCursorPos(int* outX, int* outY) {
    runOnMain(^{
        NSPoint loc = [NSEvent mouseLocation];
        NSScreen* main = [NSScreen mainScreen];
        if (!main) { *outX = -1; *outY = -1; return; }
        CGFloat primaryH = main.frame.size.height;
        *outX = (int)loc.x;
        *outY = (int)(primaryH - loc.y);
    });
}

void platformGetWindowSize(void* nsWindow, int* outLeft, int* outTop, int* outWidth, int* outHeight) {
    runOnMain(^{
        NSWindow* w = (__bridge NSWindow*)nsWindow;
        if (!w) { *outLeft = *outTop = *outWidth = *outHeight = 0; return; }
        NSRect f = [w frame];
        NSScreen* main = [NSScreen mainScreen];
        CGFloat screenH = main ? main.frame.size.height : 0;
        *outLeft = (int)f.origin.x;
        *outTop = (int)(screenH - f.origin.y - f.size.height);
        *outWidth = (int)f.size.width;
        *outHeight = (int)f.size.height;
    });
}
*/
import "C"

import (
	"unsafe"
)

// FindWindowByPID is kept for API parity but is a no-op on Wails v3 since the
// native window handle is exposed directly by the window instance.
func FindWindowByPID() (uintptr, error) {
	return 1, nil
}

func ApplyBarStyles(hwnd uintptr) {
	C.platformApplyBarStyles(unsafe.Pointer(hwnd))
}

func DockToMonitor(hwnd uintptr, mon MonitorInfo, barHeight int, appBarMode bool) {
	// NSWindow geometry uses POINTS, so pass mon.Width (points), NOT PhysWidth
	// (Retina pixels).
	C.platformDockToMonitor(unsafe.Pointer(hwnd), C.int(mon.Left), C.int(mon.Top), C.int(mon.Width), C.int(barHeight))
}

func RemoveAppBar(hwnd uintptr) {
	// No-op on macOS.
}

func GetWindowSize(hwnd uintptr) (left, top, width, height int) {
	var l, t, w, h C.int
	C.platformGetWindowSize(unsafe.Pointer(hwnd), &l, &t, &w, &h)
	return int(l), int(t), int(w), int(h)
}

func SetWindowHeight(hwnd uintptr, physHeight int) {
	l, t, w, _ := GetWindowSize(hwnd)
	C.platformDockToMonitor(unsafe.Pointer(hwnd), C.int(l), C.int(t), C.int(w), C.int(physHeight))
}

func SetOpacity(hwnd uintptr, opacity float64) {
	C.platformSetOpacity(unsafe.Pointer(hwnd), C.double(opacity))
}

// AutoHideSupported gates the slide-up auto-hide animation and the
// click-through-while-collapsed behaviour. The macOS branch now has
// MoveWindow / Show / Hide / GetCursorPos wired up, so the hover-watcher
// in app.go can drive the same expand/collapse loop as Windows.
func AutoHideSupported() bool { return true }

func GetCursorPos() (int, int) {
	var x, y C.int
	C.platformGetCursorPos(&x, &y)
	return int(x), int(y)
}

// ResetDwmFrame is a Windows-only concept; no-op elsewhere.
func ResetDwmFrame(hwnd uintptr) {}

func HideWindow(hwnd uintptr) {
	C.platformHideWindow(unsafe.Pointer(hwnd))
}

func ShowWindow(hwnd uintptr) {
	C.platformShowWindow(unsafe.Pointer(hwnd))
}

func MoveWindow(hwnd uintptr, x, y int) {
	C.platformMoveWindow(unsafe.Pointer(hwnd), C.int(x), C.int(y))
}

// SetWindowClipTop no-op on macOS for now.
func SetWindowClipTop(hwnd uintptr, width, height, topClip int) {}

func SetClickThrough(hwnd uintptr, enabled bool) {
	v := 0
	if enabled {
		v = 1
	}
	C.platformSetClickThrough(unsafe.Pointer(hwnd), C.int(v))
}
