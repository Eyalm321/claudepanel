#import <Foundation/Foundation.h>
#import <AVFoundation/AVFoundation.h>

void goDarwinPlayerCallback(void* goPlayer, const char* state, const char* errStr);

@interface DarwinPlayerObserver : NSObject
@property (nonatomic, assign) void* goPlayer;
@property (nonatomic, strong) AVPlayer* player;
@property (nonatomic, strong) AVPlayerItem* currentItem;
// Set when the item plays to its natural end. The rate-KVO branch fires a
// spurious rate==0 ("paused") right after EOS — this flag suppresses it so the
// station player sees exactly one "ended" and not a trailing "paused".
@property (nonatomic, assign) BOOL endedFired;
@end

@implementation DarwinPlayerObserver

- (instancetype)initWithGoPlayer:(void*)goPlayer {
    self = [super init];
    if (self) {
        _goPlayer = goPlayer;
    }
    return self;
}

- (void)observeValueForKeyPath:(NSString *)keyPath ofObject:(id)object change:(NSDictionary *)change context:(void *)context {
    if ([keyPath isEqualToString:@"status"]) {
        AVPlayerItemStatus status = [[change objectForKey:NSKeyValueChangeNewKey] integerValue];
        if (status == AVPlayerItemStatusReadyToPlay) {
            goDarwinPlayerCallback(_goPlayer, "playing", NULL);
        } else if (status == AVPlayerItemStatusFailed) {
            NSError* error = self.currentItem.error;
            NSString* errStr = error ? error.localizedDescription : @"AVPlayerItem status failed";
            goDarwinPlayerCallback(_goPlayer, "error", [errStr UTF8String]);
        }
    } else if ([keyPath isEqualToString:@"rate"]) {
        float rate = [[change objectForKey:NSKeyValueChangeNewKey] floatValue];
        if (rate == 0.0) {
            // Natural end already emitted "ended"; swallow the trailing rate==0
            // so we don't also report a (spurious) user pause.
            if (self.endedFired) {
                return;
            }
            goDarwinPlayerCallback(_goPlayer, "paused", NULL);
        } else {
            goDarwinPlayerCallback(_goPlayer, "playing", NULL);
        }
    }
}

- (void)itemDidPlayToEnd:(NSNotification*)notification {
    self.endedFired = YES;
    goDarwinPlayerCallback(_goPlayer, "ended", NULL);
}

- (void)itemFailedToPlay:(NSNotification*)notification {
    NSError* error = notification.userInfo[AVPlayerItemFailedToPlayToEndTimeErrorKey];
    NSString* errStr = error ? error.localizedDescription : @"Failed to play to end time";
    goDarwinPlayerCallback(_goPlayer, "error", [errStr UTF8String]);
}

- (void)stopObserving {
    if (self.currentItem) {
        @try {
            [self.currentItem removeObserver:self forKeyPath:@"status"];
        } @catch (NSException *exception) {}
        [[NSNotificationCenter defaultCenter] removeObserver:self name:AVPlayerItemFailedToPlayToEndTimeNotification object:self.currentItem];
        [[NSNotificationCenter defaultCenter] removeObserver:self name:AVPlayerItemDidPlayToEndTimeNotification object:self.currentItem];
        self.currentItem = nil;
    }
    if (self.player) {
        @try {
            [self.player removeObserver:self forKeyPath:@"rate"];
        } @catch (NSException *exception) {}
        self.player = nil;
    }
}

- (void)startObservingPlayer:(AVPlayer*)player item:(AVPlayerItem*)item {
    [self stopObserving];
    self.player = player;
    self.currentItem = item;
    self.endedFired = NO;

    [self.player addObserver:self forKeyPath:@"rate" options:NSKeyValueObservingOptionNew context:nil];
    [self.currentItem addObserver:self forKeyPath:@"status" options:NSKeyValueObservingOptionNew context:nil];

    [[NSNotificationCenter defaultCenter] addObserver:self selector:@selector(itemFailedToPlay:) name:AVPlayerItemFailedToPlayToEndTimeNotification object:self.currentItem];
    [[NSNotificationCenter defaultCenter] addObserver:self selector:@selector(itemDidPlayToEnd:) name:AVPlayerItemDidPlayToEndTimeNotification object:self.currentItem];
}

- (void)dealloc {
    [self stopObserving];
}

@end

// C wrapper functions

typedef struct {
    AVPlayer* player;
    DarwinPlayerObserver* observer;
} DarwinPlayerState;

void* createDarwinPlayer(void* goPlayer) {
    DarwinPlayerState* state = malloc(sizeof(DarwinPlayerState));
    state->player = [[AVPlayer alloc] init];
    state->observer = [[DarwinPlayerObserver alloc] initWithGoPlayer:goPlayer];
    return state;
}

void playDarwinPlayer(void* playerPtr, const char* urlStr) {
    DarwinPlayerState* state = (DarwinPlayerState*)playerPtr;
    NSString* nsUrlStr = [NSString stringWithUTF8String:urlStr];
    NSURL* url = [NSURL URLWithString:nsUrlStr];
    
    AVPlayerItem* item = [AVPlayerItem playerItemWithURL:url];
    [state->observer startObservingPlayer:state->player item:item];
    
    [state->player replaceCurrentItemWithPlayerItem:item];
    [state->player play];
}

void resumeDarwinPlayer(void* playerPtr) {
    DarwinPlayerState* state = (DarwinPlayerState*)playerPtr;
    // Resume the already-loaded item from its paused position (no
    // replaceCurrentItem, so it does not restart from the beginning).
    [state->player play];
}

void pauseDarwinPlayer(void* playerPtr) {
    DarwinPlayerState* state = (DarwinPlayerState*)playerPtr;
    [state->player pause];
}

void stopDarwinPlayer(void* playerPtr) {
    DarwinPlayerState* state = (DarwinPlayerState*)playerPtr;
    [state->player pause];
    [state->observer stopObserving];
    [state->player replaceCurrentItemWithPlayerItem:nil];
}

void setVolumeDarwinPlayer(void* playerPtr, float vol) {
    DarwinPlayerState* state = (DarwinPlayerState*)playerPtr;
    state->player.volume = vol;
}

void freeDarwinPlayer(void* playerPtr) {
    if (!playerPtr) return;
    DarwinPlayerState* state = (DarwinPlayerState*)playerPtr;
    [state->player pause];
    [state->observer stopObserving];
    state->player = nil;
    state->observer = nil;
    free(state);
}
