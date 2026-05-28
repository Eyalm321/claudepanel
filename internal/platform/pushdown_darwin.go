//go:build darwin

package platform

/*
#cgo darwin CFLAGS: -x objective-c -fobjc-arc
#cgo darwin LDFLAGS: -framework Cocoa -framework ApplicationServices
#import <Cocoa/Cocoa.h>
#import <ApplicationServices/ApplicationServices.h>

// Global State
static dispatch_once_t onceToken;
static dispatch_queue_t pushdownQueue;
static NSMutableDictionary* observedApps;
static BOOL pushdownEnabled = NO;

// Stored geometry
static int barLeft = 0;
static int barTop = 0;
static int barWidth = 0;
static int barMonHeight = 0;
static int barHeightVal = 0;

// Diagnostics
static int pushesCount = 0;
static NSString* lastErrStr = nil;

// Active timers for throttling window drag/resize
static NSMutableDictionary* activeTimers = nil;

// Recheck timer for AX permission
static dispatch_source_t trustTimer = nil;
static int trustTimerTicks = 0;

// Forward declarations
static void sweepAllRunningApps(void);
static void detachObserverFromApp(pid_t pid);
static void attachObserverToApp(pid_t pid, NSString* bundleID);

@interface PushdownWorkspaceObserver : NSObject
+ (instancetype)sharedInstance;
- (void)startObserving;
- (void)stopObserving;
@end

@implementation PushdownWorkspaceObserver

+ (instancetype)sharedInstance {
    static PushdownWorkspaceObserver* instance = nil;
    static dispatch_once_t onceTokenWorkspace;
    dispatch_once(&onceTokenWorkspace, ^{
        instance = [[PushdownWorkspaceObserver alloc] init];
    });
    return instance;
}

- (void)startObserving {
    NSNotificationCenter* nc = [[NSWorkspace sharedWorkspace] notificationCenter];
    [nc removeObserver:self];
    [nc addObserver:self
           selector:@selector(appLaunched:)
               name:NSWorkspaceDidLaunchApplicationNotification
             object:nil];
    [nc addObserver:self
           selector:@selector(appTerminated:)
               name:NSWorkspaceDidTerminateApplicationNotification
             object:nil];
}

- (void)stopObserving {
    NSNotificationCenter* nc = [[NSWorkspace sharedWorkspace] notificationCenter];
    [nc removeObserver:self];
}

- (void)appLaunched:(NSNotification*)notif {
    NSRunningApplication* app = notif.userInfo[NSWorkspaceApplicationKey];
    if (app && app.activationPolicy == NSApplicationActivationPolicyRegular) {
        pid_t pid = app.processIdentifier;
        NSString* bid = app.bundleIdentifier;
        dispatch_async(pushdownQueue, ^{
            attachObserverToApp(pid, bid);
        });
    }
}

- (void)appTerminated:(NSNotification*)notif {
    NSRunningApplication* app = notif.userInfo[NSWorkspaceApplicationKey];
    if (app) {
        pid_t pid = app.processIdentifier;
        dispatch_async(pushdownQueue, ^{
            detachObserverFromApp(pid);
        });
    }
}

@end

static void initPushdownIfNeeded(void) {
    dispatch_once(&onceToken, ^{
        pushdownQueue = dispatch_queue_create("com.claudepanel.pushdown", DISPATCH_QUEUE_SERIAL);
        observedApps = [NSMutableDictionary dictionary];
        activeTimers = [NSMutableDictionary dictionary];
    });
}

static void pushWindow(AXUIElementRef win) {
    if (!pushdownEnabled) return;

    // 1. Skip if kAXRoleAttribute != kAXWindowRole
    CFTypeRef role = NULL;
    if (AXUIElementCopyAttributeValue(win, kAXRoleAttribute, &role) == kAXErrorSuccess) {
        if (role) {
            BOOL isWindow = CFEqual(role, kAXWindowRole);
            CFRelease(role);
            if (!isWindow) return;
        } else {
            return;
        }
    } else {
        return;
    }

    // 2. Skip if "AXFullScreen" == true
    CFTypeRef fullscreen = NULL;
    if (AXUIElementCopyAttributeValue(win, CFSTR("AXFullScreen"), &fullscreen) == kAXErrorSuccess) {
        if (fullscreen) {
            BOOL isFS = NO;
            if (CFGetTypeID(fullscreen) == CFBooleanGetTypeID()) {
                isFS = CFBooleanGetValue(fullscreen);
            }
            CFRelease(fullscreen);
            if (isFS) return;
        }
    }

    // 3. Skip if kAXMinimizedAttribute == true
    CFTypeRef minimized = NULL;
    if (AXUIElementCopyAttributeValue(win, kAXMinimizedAttribute, &minimized) == kAXErrorSuccess) {
        if (minimized) {
            BOOL isMin = NO;
            if (CFGetTypeID(minimized) == CFBooleanGetTypeID()) {
                isMin = CFBooleanGetValue(minimized);
            }
            CFRelease(minimized);
            if (isMin) return;
        }
    }

    // 4. Read kAXPositionAttribute -> CGPoint p, kAXSizeAttribute -> CGSize s
    CGPoint p = CGPointZero;
    CFTypeRef posVal = NULL;
    if (AXUIElementCopyAttributeValue(win, kAXPositionAttribute, &posVal) == kAXErrorSuccess) {
        if (posVal) {
            if (CFGetTypeID(posVal) == AXValueGetTypeID()) {
                AXValueGetValue(posVal, kAXValueTypeCGPoint, &p);
            }
            CFRelease(posVal);
        }
    } else {
        return;
    }

    CGSize s = CGSizeZero;
    CFTypeRef sizeVal = NULL;
    if (AXUIElementCopyAttributeValue(win, kAXSizeAttribute, &sizeVal) == kAXErrorSuccess) {
        if (sizeVal) {
            if (CFGetTypeID(sizeVal) == AXValueGetTypeID()) {
                AXValueGetValue(sizeVal, kAXValueTypeCGSize, &s);
            }
            CFRelease(sizeVal);
        }
    } else {
        return;
    }

    // 5. Monitor filter: only push if window's center is inside the bar's monitor rect
    CGFloat cx = p.x + s.width / 2.0;
    CGFloat cy = p.y + s.height / 2.0;
    if (cx < barLeft || cx >= barLeft + barWidth || cy < barTop || cy >= barTop + barMonHeight) {
        return;
    }

    // 6. Pushdown calculations
    CGFloat barBottom = barTop + barHeightVal;
    if (p.y >= barBottom) {
        // Under the bar, nothing to do
        return;
    }

    CGFloat delta = barBottom - p.y;
    pushesCount++;

    CGPoint newPos = CGPointMake(p.x, barBottom);
    AXValueRef newPosVal = AXValueCreate(kAXValueTypeCGPoint, &newPos);
    if (newPosVal) {
        AXUIElementSetAttributeValue(win, kAXPositionAttribute, newPosVal);
        CFRelease(newPosVal);
    }

    CGFloat newH = s.height - delta;
    if (newH < 200) newH = 200;
    CGSize newSize = CGSizeMake(s.width, newH);
    AXValueRef newSizeVal = AXValueCreate(kAXValueTypeCGSize, &newSize);
    if (newSizeVal) {
        AXUIElementSetAttributeValue(win, kAXSizeAttribute, newSizeVal);
        CFRelease(newSizeVal);
    }

    // Read back and retry once if mismatch > 2px (e.g. Electron layout quirk)
    CGPoint checkP = CGPointZero;
    CFTypeRef checkPosVal = NULL;
    BOOL retry = NO;
    if (AXUIElementCopyAttributeValue(win, kAXPositionAttribute, &checkPosVal) == kAXErrorSuccess) {
        if (checkPosVal) {
            if (CFGetTypeID(checkPosVal) == AXValueGetTypeID()) {
                AXValueGetValue(checkPosVal, kAXValueTypeCGPoint, &checkP);
            }
            CFRelease(checkPosVal);
            if (fabs(checkP.y - barBottom) > 2.0) {
                retry = YES;
            }
        }
    }

    if (retry) {
        // The block is queued for +100ms but the caller's CFRetain on `win`
        // (held by scheduleThrottledPush's timer handler) is released as soon
        // as pushWindow returns — so we have to own a retain for the lifetime
        // of this deferred block ourselves. Without it, `win` is freed before
        // the dispatch_after runs and the AXUIElement* calls below dereference
        // freed memory (= crash whenever the retry path triggers, i.e. exactly
        // when a window is being dragged toward the bar edge).
        CFRetain(win);
        dispatch_after(dispatch_time(DISPATCH_TIME_NOW, (int64_t)(100 * NSEC_PER_MSEC)), pushdownQueue, ^{
            if (!pushdownEnabled) {
                CFRelease(win);
                return;
            }

            CGPoint p2 = CGPointZero;
            CFTypeRef posVal2 = NULL;
            if (AXUIElementCopyAttributeValue(win, kAXPositionAttribute, &posVal2) == kAXErrorSuccess) {
                if (posVal2) {
                    if (CFGetTypeID(posVal2) == AXValueGetTypeID()) {
                        AXValueGetValue(posVal2, kAXValueTypeCGPoint, &p2);
                    }
                    CFRelease(posVal2);
                }
            } else { CFRelease(win); return; }

            CGSize s2 = CGSizeZero;
            CFTypeRef sizeVal2 = NULL;
            if (AXUIElementCopyAttributeValue(win, kAXSizeAttribute, &sizeVal2) == kAXErrorSuccess) {
                if (sizeVal2) {
                    if (CFGetTypeID(sizeVal2) == AXValueGetTypeID()) {
                        AXValueGetValue(sizeVal2, kAXValueTypeCGSize, &s2);
                    }
                    CFRelease(sizeVal2);
                }
            } else { CFRelease(win); return; }

            CGFloat cx2 = p2.x + s2.width / 2.0;
            CGFloat cy2 = p2.y + s2.height / 2.0;
            if (cx2 < barLeft || cx2 >= barLeft + barWidth || cy2 < barTop || cy2 >= barTop + barMonHeight) {
                CFRelease(win);
                return;
            }
            if (p2.y >= barBottom) { CFRelease(win); return; }

            CGFloat delta2 = barBottom - p2.y;
            CGPoint newPos2 = CGPointMake(p2.x, barBottom);
            AXValueRef newPosVal2 = AXValueCreate(kAXValueTypeCGPoint, &newPos2);
            if (newPosVal2) {
                AXUIElementSetAttributeValue(win, kAXPositionAttribute, newPosVal2);
                CFRelease(newPosVal2);
            }

            CGFloat newH2 = s2.height - delta2;
            if (newH2 < 200) newH2 = 200;
            CGSize newSize2 = CGSizeMake(s2.width, newH2);
            AXValueRef newSizeVal2 = AXValueCreate(kAXValueTypeCGSize, &newSize2);
            if (newSizeVal2) {
                AXUIElementSetAttributeValue(win, kAXSizeAttribute, newSizeVal2);
                CFRelease(newSizeVal2);
            }

            CFRelease(win);
        });
    }
}

static void sweepAppWindows(AXUIElementRef app) {
    CFTypeRef windowsVal = NULL;
    if (AXUIElementCopyAttributeValue(app, kAXWindowsAttribute, &windowsVal) == kAXErrorSuccess) {
        if (windowsVal) {
            if (CFGetTypeID(windowsVal) == CFArrayGetTypeID()) {
                CFArrayRef windowList = (CFArrayRef)windowsVal;
                CFIndex count = CFArrayGetCount(windowList);
                for (CFIndex i = 0; i < count; i++) {
                    AXUIElementRef win = (AXUIElementRef)CFArrayGetValueAtIndex(windowList, i);
                    pushWindow(win);
                }
            }
            CFRelease(windowsVal);
        }
    }
}

static void scheduleThrottledPush(pid_t pid, AXUIElementRef win) {
    NSNumber* key = @(pid);
    dispatch_source_t timer = activeTimers[key];
    if (timer) {
        dispatch_source_cancel(timer);
    }

    timer = dispatch_source_create(DISPATCH_SOURCE_TYPE_TIMER, 0, 0, pushdownQueue);
    activeTimers[key] = timer;

    CFRetain(win);
    dispatch_source_set_event_handler(timer, ^{
        if (pushdownEnabled) {
            pushWindow(win);
        }
        CFRelease(win);
        dispatch_source_cancel(timer);
        [activeTimers removeObjectForKey:key];
    });

    dispatch_time_t startTime = dispatch_time(DISPATCH_TIME_NOW, 50 * NSEC_PER_MSEC);
    dispatch_source_set_timer(timer, startTime, DISPATCH_TIME_FOREVER, 10 * NSEC_PER_MSEC);
    dispatch_resume(timer);
}

static void axCallback(AXObserverRef observer, AXUIElementRef element, CFStringRef notification, void* refcon) {
    CFRetain(element);
    dispatch_async(pushdownQueue, ^{
        if (pushdownEnabled) {
            pid_t pid = 0;
            AXUIElementGetPid(element, &pid);
            if (CFEqual(notification, kAXWindowMovedNotification) || CFEqual(notification, kAXWindowResizedNotification)) {
                scheduleThrottledPush(pid, element);
            } else {
                pushWindow(element);
            }
        }
        CFRelease(element);
    });
}

static void attachObserverToApp(pid_t pid, NSString* bundleID) {
    if (!pushdownEnabled) return;

    if (pid == getpid()) return;

    NSArray* skipList = @[
        @"com.apple.dock",
        @"com.apple.systemuiserver",
        @"com.apple.controlcenter",
        @"com.apple.notificationcenterui",
        @"com.apple.WindowManager"
    ];
    if (bundleID && [skipList containsObject:bundleID]) {
        return;
    }

    NSNumber* key = @(pid);
    if (observedApps[key]) {
        return;
    }

    AXUIElementRef appElem = AXUIElementCreateApplication(pid);
    if (!appElem) return;

    AXObserverRef obs = NULL;
    AXError err = AXObserverCreate(pid, axCallback, &obs);
    if (err != kAXErrorSuccess || !obs) {
        CFRelease(appElem);
        lastErrStr = [NSString stringWithFormat:@"AXObserverCreate failed for pid %d: %d", (int)pid, (int)err];
        return;
    }

    // Subscribe to ONLY the events that actually warrant a push. The earlier
    // version also listened for window-created, focused-window-changed, and
    // (mis-spelled) deminimized — those fire constantly from normal desktop
    // interactions (every click on the desktop fires AXFocusedWindowChanged
    // from Finder), and each one fell to pushWindow on whatever element the
    // notification carried. Some of those elements are special (Finder's
    // desktop "window", brand-new windows that haven't finished initialising)
    // and AX calls on them have been the proximate cause of crashes whenever
    // the user clicks the desktop / opens a new window. Move + Resize are
    // the only events the pushdown contract actually depends on, and they
    // already cover the dragging / maximise paths.
    NSArray* notifications = @[
        (__bridge NSString*)kAXWindowMovedNotification,
        (__bridge NSString*)kAXWindowResizedNotification,
    ];

    for (NSString* notif in notifications) {
        AXObserverAddNotification(obs, appElem, (__bridge CFStringRef)notif, NULL);
    }

    CFRunLoopSourceRef src = AXObserverGetRunLoopSource(obs);
    if (src) {
        CFRunLoopAddSource(CFRunLoopGetMain(), src, kCFRunLoopDefaultMode);
    }

    observedApps[key] = [NSValue valueWithPointer:obs];
    sweepAppWindows(appElem);

    CFRelease(appElem);
}

static void detachObserverFromApp(pid_t pid) {
    NSNumber* key = @(pid);
    NSValue* val = observedApps[key];
    if (val) {
        AXObserverRef obs = [val pointerValue];
        if (obs) {
            CFRunLoopSourceRef src = AXObserverGetRunLoopSource(obs);
            if (src) {
                CFRunLoopRemoveSource(CFRunLoopGetMain(), src, kCFRunLoopDefaultMode);
            }
            CFRelease(obs);
        }
        [observedApps removeObjectForKey:key];
    }

    dispatch_source_t timer = activeTimers[key];
    if (timer) {
        dispatch_source_cancel(timer);
        [activeTimers removeObjectForKey:key];
    }
}

static void sweepAllRunningApps(void) {
    // [NSWorkspace sharedWorkspace] runningApplications is documented as
    // main-thread on modern macOS; calling it from the pushdown queue has
    // shown up in crash reports for some users. Snapshot (pid, bundleId)
    // pairs on the main thread, then attach observers back on our queue.
    __block NSArray<NSDictionary*>* snapshot = nil;
    dispatch_sync(dispatch_get_main_queue(), ^{
        NSMutableArray<NSDictionary*>* items = [NSMutableArray array];
        for (NSRunningApplication* app in [[NSWorkspace sharedWorkspace] runningApplications]) {
            if (app.activationPolicy != NSApplicationActivationPolicyRegular) continue;
            NSString* bid = app.bundleIdentifier ?: @"";
            [items addObject:@{ @"pid": @(app.processIdentifier), @"bid": bid }];
        }
        snapshot = items;
    });
    for (NSDictionary* item in snapshot) {
        pid_t pid = (pid_t)[item[@"pid"] intValue];
        NSString* bid = item[@"bid"];
        attachObserverToApp(pid, bid);
    }
}

static void startTrustTimer(void) {
    if (trustTimer) {
        dispatch_source_cancel(trustTimer);
        trustTimer = nil;
    }
    trustTimerTicks = 0;
    trustTimer = dispatch_source_create(DISPATCH_SOURCE_TYPE_TIMER, 0, 0, pushdownQueue);

    dispatch_source_set_event_handler(trustTimer, ^{
        trustTimerTicks++;
        if (AXIsProcessTrusted()) {
            dispatch_async(dispatch_get_main_queue(), ^{
                [[PushdownWorkspaceObserver sharedInstance] startObserving];
            });
            sweepAllRunningApps();
            dispatch_source_cancel(trustTimer);
            trustTimer = nil;
        } else if (trustTimerTicks >= 150) { // 5 minutes
            dispatch_source_cancel(trustTimer);
            trustTimer = nil;
            lastErrStr = @"Accessibility permission check timed out (5 minutes)";
        }
    });

    dispatch_time_t startTime = dispatch_time(DISPATCH_TIME_NOW, 2000 * NSEC_PER_MSEC);
    dispatch_source_set_timer(trustTimer, startTime, 2000 * NSEC_PER_MSEC, 100 * NSEC_PER_MSEC);
    dispatch_resume(trustTimer);
}

// C-API wrappers for Go
static int platformAXTrusted(void) {
    return AXIsProcessTrusted() ? 1 : 0;
}

static int platformAXRequestTrust(void) {
    NSDictionary* options = @{(__bridge id)kAXTrustedCheckOptionPrompt: @YES};
    return AXIsProcessTrustedWithOptions((__bridge CFDictionaryRef)options) ? 1 : 0;
}

static int platformPushdownEnable(int left, int top, int width, int height, int barHeight) {
    initPushdownIfNeeded();

    __block BOOL trusted = NO;
    dispatch_sync(pushdownQueue, ^{
        pushdownEnabled = YES;
        barLeft = left;
        barTop = top;
        barWidth = width;
        barMonHeight = height;
        barHeightVal = barHeight;

        trusted = AXIsProcessTrusted();
        if (trusted) {
            if (trustTimer) {
                dispatch_source_cancel(trustTimer);
                trustTimer = nil;
            }
            dispatch_async(dispatch_get_main_queue(), ^{
                [[PushdownWorkspaceObserver sharedInstance] startObserving];
            });
            sweepAllRunningApps();
        } else {
            dispatch_async(dispatch_get_main_queue(), ^{
                NSDictionary* options = @{(__bridge id)kAXTrustedCheckOptionPrompt: @YES};
                AXIsProcessTrustedWithOptions((__bridge CFDictionaryRef)options);
            });
            startTrustTimer();
        }
    });
    return trusted ? 1 : 0;
}

static void platformPushdownDisable(void) {
    initPushdownIfNeeded();

    dispatch_sync(pushdownQueue, ^{
        pushdownEnabled = NO;
        if (trustTimer) {
            dispatch_source_cancel(trustTimer);
            trustTimer = nil;
        }

        dispatch_async(dispatch_get_main_queue(), ^{
            [[PushdownWorkspaceObserver sharedInstance] stopObserving];
        });

        NSArray* pids = [observedApps allKeys];
        for (NSNumber* pidVal in pids) {
            detachObserverFromApp([pidVal intValue]);
        }
    });
}

static void platformPushdownReconfigure(int left, int top, int width, int height, int barHeight) {
    initPushdownIfNeeded();

    dispatch_sync(pushdownQueue, ^{
        barLeft = left;
        barTop = top;
        barWidth = width;
        barMonHeight = height;
        barHeightVal = barHeight;

        if (pushdownEnabled && AXIsProcessTrusted()) {
            sweepAllRunningApps();
        }
    });
}

typedef struct {
    int enabled;
    int trusted;
    int observedApps;
    int pushesThisSession;
    const char* lastError;
} CPushdownStats;

static CPushdownStats platformPushdownStats(void) {
    __block CPushdownStats s = {0};
    initPushdownIfNeeded();

    dispatch_sync(pushdownQueue, ^{
        s.enabled = pushdownEnabled ? 1 : 0;
        s.trusted = AXIsProcessTrusted() ? 1 : 0;
        s.observedApps = (int)[observedApps count];
        s.pushesThisSession = pushesCount;
        s.lastError = lastErrStr ? [lastErrStr UTF8String] : "";
    });
    return s;
}
*/
import "C"

import (
	"errors"
)

// PushdownEnable starts window observation and window adjustment for the given monitor.
// If accessibility permission is not granted, it triggers the OS prompt and starts a background
// polling timer to check for permission changes.
//
// The `top` we hand the C side is mon.Top + WorkTopOffset — i.e., the bar's
// *resting* top edge below the menu bar. The pushdown uses that to compute
// barBottom; if we passed raw mon.Top (= 0 on macOS) it would only push
// windows past Y=BarHeight, leaving them overlapping the slice of the bar
// that lives below the menu bar.
func PushdownEnable(mon MonitorInfo, barHeight int) error {
	res := C.platformPushdownEnable(
		C.int(mon.Left),
		C.int(int(mon.Top)+mon.WorkTopOffset),
		C.int(mon.Width),
		C.int(mon.Height),
		C.int(barHeight),
	)
	if res == 0 {
		return errors.New("accessibility permission not granted; prompting user")
	}
	return nil
}

// PushdownDisable stops window observation and releases all observers.
func PushdownDisable() {
	C.platformPushdownDisable()
}

// PushdownReconfigure updates the active monitor geometry for window adjustment.
func PushdownReconfigure(mon MonitorInfo, barHeight int) {
	C.platformPushdownReconfigure(
		C.int(mon.Left),
		C.int(int(mon.Top)+mon.WorkTopOffset),
		C.int(mon.Width),
		C.int(mon.Height),
		C.int(barHeight),
	)
}

// AXTrusted performs a silent check to determine if accessibility permission is trusted.
func AXTrusted() bool {
	return C.platformAXTrusted() != 0
}

// AXRequestTrust prompts the OS accessibility grant dialog.
func AXRequestTrust() bool {
	return C.platformAXRequestTrust() != 0
}

// GetPushdownStats returns diagnostics about the active window pushdown session.
func GetPushdownStats() PushdownStats {
	cStats := C.platformPushdownStats()
	return PushdownStats{
		Enabled:           cStats.enabled != 0,
		Trusted:           cStats.trusted != 0,
		ObservedApps:      int(cStats.observedApps),
		PushesThisSession: int(cStats.pushesThisSession),
		LastError:         C.GoString(cStats.lastError),
	}
}
