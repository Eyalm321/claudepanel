//go:build darwin

package platform

/*
#cgo darwin CFLAGS: -x objective-c -fobjc-arc
#cgo darwin LDFLAGS: -framework Cocoa
#import <Cocoa/Cocoa.h>

static NSWindow* findOurWindow(void) {
    NSArray<NSWindow*>* windows = [NSApp windows];
    for (NSWindow* w in windows) {
        if ([w isVisible] && [w title] != nil) {
            return w;
        }
    }
    if ([windows count] > 0) return [windows objectAtIndex:0];
    return nil;
}

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

void platformApplyBarStyles(void) {
    runOnMain(^{
        NSWindow* w = findOurWindow();
        if (!w) return;
        [w setStyleMask:NSWindowStyleMaskBorderless];
        [w setLevel:NSStatusWindowLevel];
        [w setCollectionBehavior:(NSWindowCollectionBehaviorCanJoinAllSpaces | NSWindowCollectionBehaviorStationary)];
        [w setHasShadow:NO];
        [w setMovable:NO];
    });
}

void platformSetOpacity(double opacity) {
    runOnMain(^{
        NSWindow* w = findOurWindow();
        if (!w) return;
        double a = opacity;
        if (a < 0) a = 0;
        if (a > 1) a = 1;
        [w setAlphaValue:a];
    });
}

void platformSetClickThrough(int enabled) {
    runOnMain(^{
        NSWindow* w = findOurWindow();
        if (!w) return;
        [w setIgnoresMouseEvents:(enabled ? YES : NO)];
    });
}

void platformDockToMonitor(int left, int top, int width, int height) {
    runOnMain(^{
        NSWindow* w = findOurWindow();
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
    });
}

void platformGetWindowSize(int* outLeft, int* outTop, int* outWidth, int* outHeight) {
    runOnMain(^{
        NSWindow* w = findOurWindow();
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
	"fmt"
	"os"
)

// On macOS the "hwnd" is unused — we look up the window via NSApp.windows.
// We still keep the uintptr return for API parity.
func FindWindowByPID() (uintptr, error) {
	if os.Getpid() == 0 {
		return 0, fmt.Errorf("no process")
	}
	return 1, nil // sentinel: non-zero so existing callers proceed
}

func ApplyBarStyles(hwnd uintptr) {
	C.platformApplyBarStyles()
}

func DockToMonitor(hwnd uintptr, mon MonitorInfo, barHeight int, appBarMode bool) {
	// macOS has no AppBar equivalent — Cocoa always renders the system menu
	// bar above every window level. The Objective-C side positions us at
	// [NSScreen visibleFrame] (the area below the menu bar) instead of
	// fighting it. NSWindow geometry uses POINTS, so pass mon.Width (points),
	// NOT PhysWidth (Retina pixels) — otherwise on a 2x display the window
	// would be twice as wide as the screen.
	C.platformDockToMonitor(C.int(mon.Left), C.int(mon.Top), C.int(mon.Width), C.int(barHeight))
}

func RemoveAppBar(hwnd uintptr) {
	// No-op on macOS.
}

func GetWindowSize(hwnd uintptr) (left, top, width, height int) {
	var l, t, w, h C.int
	C.platformGetWindowSize(&l, &t, &w, &h)
	return int(l), int(t), int(w), int(h)
}

func SetWindowHeight(hwnd uintptr, physHeight int) {
	l, t, w, _ := GetWindowSize(hwnd)
	C.platformDockToMonitor(C.int(l), C.int(t), C.int(w), C.int(physHeight))
}

func SetOpacity(hwnd uintptr, opacity float64) {
	C.platformSetOpacity(C.double(opacity))
}

// GetCursorPos is a stub on macOS; the hover-watcher is Windows-only for v1.
func GetCursorPos() (int, int) { return -1, -1 }

// ResetDwmFrame is a Windows-only concept; no-op elsewhere.
func ResetDwmFrame(hwnd uintptr) {}

// HideWindow / ShowWindow no-op on macOS for now (auto-hide is Windows-only v1).
func HideWindow(hwnd uintptr) {}
func ShowWindow(hwnd uintptr) {}

// MoveWindow no-op on macOS for now (slide animation is Windows-only v1).
func MoveWindow(hwnd uintptr, x, y int) {}

// SetWindowClipTop no-op on macOS for now.
func SetWindowClipTop(hwnd uintptr, width, height, topClip int) {}

func SetClickThrough(hwnd uintptr, enabled bool) {
	v := 0
	if enabled {
		v = 1
	}
	C.platformSetClickThrough(C.int(v))
}
