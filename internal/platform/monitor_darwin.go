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
    CGFloat scale = s.backingScaleFactor;
    // Convert NSScreen's bottom-left origin to our top-left convention,
    // flipped against the primary (zero-origin) screen height.
    CGFloat primaryH = mainScreen ? mainScreen.frame.size.height : frame.size.height;
    m.left = (int)frame.origin.x;
    m.top = (int)(primaryH - frame.origin.y - frame.size.height);
    m.width = (int)frame.size.width;
    m.height = (int)frame.size.height;
    m.physWidth = (int)(frame.size.width * scale);
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
			Index:     i,
			Left:      int32(cm.left),
			Top:       int32(cm.top),
			Width:     int(cm.width),
			Height:    int(cm.height),
			PhysWidth: int(cm.physWidth),
			DpiScale:  float64(cm.dpiScale),
			IsPrimary: cm.isPrimary != 0,
			Name:      C.GoString(cm.name),
		})
	}
	return out
}
