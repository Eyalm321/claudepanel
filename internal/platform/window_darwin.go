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

void platformApplyBarStyles(void) {
    NSWindow* w = findOurWindow();
    if (!w) return;
    [w setStyleMask:NSWindowStyleMaskBorderless];
    [w setLevel:NSStatusWindowLevel];
    [w setCollectionBehavior:(NSWindowCollectionBehaviorCanJoinAllSpaces | NSWindowCollectionBehaviorStationary)];
    [w setHasShadow:NO];
    [w setMovable:NO];
}

void platformSetOpacity(double opacity) {
    NSWindow* w = findOurWindow();
    if (!w) return;
    if (opacity < 0) opacity = 0;
    if (opacity > 1) opacity = 1;
    [w setAlphaValue:opacity];
}

void platformSetClickThrough(int enabled) {
    NSWindow* w = findOurWindow();
    if (!w) return;
    [w setIgnoresMouseEvents:(enabled ? YES : NO)];
}

void platformDockToMonitor(int left, int top, int width, int height) {
    NSWindow* w = findOurWindow();
    if (!w) return;
    // macOS uses a bottom-left origin coordinate system. We receive top-left
    // pixel coordinates (matching our cross-platform MonitorInfo convention),
    // so flip Y against the primary screen height.
    NSScreen* main = [NSScreen mainScreen];
    CGFloat screenH = main ? main.frame.size.height : 0;
    CGFloat y = screenH - (CGFloat)(top + height);
    NSRect frame = NSMakeRect((CGFloat)left, y, (CGFloat)width, (CGFloat)height);
    [w setFrame:frame display:YES];
}

void platformGetWindowSize(int* outLeft, int* outTop, int* outWidth, int* outHeight) {
    NSWindow* w = findOurWindow();
    if (!w) { *outLeft = *outTop = *outWidth = *outHeight = 0; return; }
    NSRect f = [w frame];
    NSScreen* main = [NSScreen mainScreen];
    CGFloat screenH = main ? main.frame.size.height : 0;
    *outLeft = (int)f.origin.x;
    *outTop = (int)(screenH - f.origin.y - f.size.height);
    *outWidth = (int)f.size.width;
    *outHeight = (int)f.size.height;
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
	// macOS has no AppBar equivalent — NSWindow.level = NSStatusWindowLevel
	// already floats above normal windows. We position to the top of the
	// chosen NSScreen but cannot reserve space the way SHAppBarMessage does.
	width := mon.PhysWidth
	if width == 0 {
		width = mon.Width
	}
	C.platformDockToMonitor(C.int(mon.Left), C.int(mon.Top), C.int(width), C.int(barHeight))
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
