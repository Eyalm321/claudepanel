//go:build darwin

package platform

/*
#cgo darwin CFLAGS: -x objective-c -fobjc-arc
#cgo darwin LDFLAGS: -framework Cocoa
#import <Cocoa/Cocoa.h>

typedef struct {
    int left;
    int top;
    int width;
    int height;
    int physWidth;
    int workTopOffset;
    double dpiScale;
    int isPrimary;
    const char* name;
} CMonitor;

int platformScreenCount(void) {
    return (int)[[NSScreen screens] count];
}

CMonitor platformGetScreen(int idx) {
    CMonitor m = {0};
    NSArray<NSScreen*>* screens = [NSScreen screens];
    if (idx < 0 || idx >= (int)[screens count]) return m;
    NSScreen* s = [screens objectAtIndex:idx];
    NSScreen* mainScreen = [NSScreen mainScreen];
    NSRect frame = s.frame;
    NSRect visible = s.visibleFrame;
    CGFloat scale = s.backingScaleFactor;
    // Convert NSScreen's bottom-left origin to our top-left convention,
    // flipped against the primary (zero-origin) screen height.
    CGFloat primaryH = mainScreen ? mainScreen.frame.size.height : frame.size.height;
    m.left = (int)frame.origin.x;
    m.top = (int)(primaryH - frame.origin.y - frame.size.height);
    m.width = (int)frame.size.width;
    m.height = (int)frame.size.height;
    // Menu-bar height for THIS screen. NSScreen.visibleFrame already excludes
    // both the menu bar and the Dock; the menu bar is exactly the slice above
    // (visible.origin.y + visible.size.height) inside frame. Secondary screens
    // without a menu bar end up with workTopOffset = 0.
    int menuBarH = (int)(frame.size.height -
                         (visible.origin.y + visible.size.height));
    if (menuBarH < 0) menuBarH = 0;
    m.workTopOffset = menuBarH;
    // On macOS, all the consumers of PhysWidth (window framing, hover hit
    // detection) work in points, not pixels — NSWindow.setFrame takes points,
    // NSEvent.mouseLocation returns points. Reporting points here keeps the
    // shared code in app.go correct on Retina displays without sprinkling
    // OS checks. Windows still reports true pixels.
    m.physWidth = (int)frame.size.width;
    m.dpiScale = (double)scale;
    m.isPrimary = [s isEqual:mainScreen] ? 1 : 0;
    NSString* name = [s respondsToSelector:@selector(localizedName)] ? [s performSelector:@selector(localizedName)] : nil;
    m.name = name ? [name UTF8String] : "Display";
    return m;
}
*/
import "C"

func GetMonitors() []MonitorInfo {
	count := int(C.platformScreenCount())
	out := make([]MonitorInfo, 0, count)
	for i := 0; i < count; i++ {
		cm := C.platformGetScreen(C.int(i))
		out = append(out, MonitorInfo{
			Index:         i,
			Left:          int32(cm.left),
			Top:           int32(cm.top),
			Width:         int(cm.width),
			Height:        int(cm.height),
			PhysWidth:     int(cm.physWidth),
			WorkTopOffset: int(cm.workTopOffset),
			DpiScale:      float64(cm.dpiScale),
			IsPrimary:     cm.isPrimary != 0,
			Name:          C.GoString(cm.name),
		})
	}
	return out
}
