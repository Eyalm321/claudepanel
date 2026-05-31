package audio

import (
	"context"
	"log"
	"sync"
)

// ResolvedTrack mirrors radio.ResolvedTrack so the audio package stays free of
// a dependency on the radio package (which imports kkdai/youtube).
type ResolvedTrack struct {
	URL    string
	IsLive bool
}

type StreamResolver interface {
	Resolve(ctx context.Context, videoID string, forceRefresh bool) (ResolvedTrack, error)
}

type Controller struct {
	mu            sync.Mutex
	player        Player
	resolver      StreamResolver
	emit          func(Event)
	activeVideoID string
	activeURL     string
	currentState  State
	curIsLive     bool
	retried       bool
}

func NewController(resolver StreamResolver, emit func(Event)) (*Controller, error) {
	c := &Controller{
		resolver: resolver,
		emit:     emit,
	}

	player, err := New(c.handlePlayerEvent)
	if err != nil {
		return nil, err
	}
	c.player = player
	return c, nil
}

func (c *Controller) handlePlayerEvent(ev Event) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Suppress duplicate idle/unaltered state transitions to avoid log/event spam
	if ev.State == c.currentState && ev.Err == "" {
		return
	}

	log.Printf("[audio] Player event received: State=%s, Err=%s", ev.State, ev.Err)

	if ev.State == StateError {
		if !c.retried && c.activeVideoID != "" {
			c.retried = true
			log.Printf("[audio] Player encountered error: %s. Attempting once-off stream URL refresh...", ev.Err)

			// Retrying asynchronously
			go func(videoID string) {
				fresh, err := c.resolver.Resolve(context.Background(), videoID, true)
				if err != nil {
					log.Printf("[audio] Refresh resolve failed on retry: %v", err)
					c.emit(Event{State: StateError, VideoID: videoID, Err: err.Error()})
					return
				}

				c.mu.Lock()
				if c.activeVideoID != videoID {
					c.mu.Unlock()
					return // target video changed in the meantime
				}
				c.activeURL = fresh.URL
				c.curIsLive = fresh.IsLive
				player := c.player
				c.mu.Unlock()

				if player != nil {
					log.Printf("[audio] Replaying refreshed URL: %s", fresh.URL)
					if err := player.Play(fresh.URL); err != nil {
						log.Printf("[audio] Retry play failed: %v", err)
						c.emit(Event{State: StateError, VideoID: videoID, Err: err.Error()})
					}
				}
			}(c.activeVideoID)
			return
		}
	}

	if ev.State == StateLoading {
		if c.currentState == StatePlaying {
			log.Printf("[audio] Suppressing StateLoading event because player is already in StatePlaying")
			return
		}
	}

	if ev.State != "" {
		c.currentState = ev.State
	}
	if ev.VideoID == "" {
		ev.VideoID = c.activeVideoID
	}
	c.emit(ev)
}

func (c *Controller) PlayVideo(ctx context.Context, videoID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	log.Printf("[audio] PlayVideo called for video %s", videoID)

	if c.activeVideoID == videoID && c.currentState == StatePlaying {
		log.Printf("[audio] Already playing video %s", videoID)
		return nil
	}

	c.activeVideoID = videoID
	c.currentState = StateLoading
	c.retried = false
	c.emit(Event{State: StateLoading, VideoID: videoID})

	log.Printf("[audio] Resolving stream URL for video %s...", videoID)
	track, err := c.resolver.Resolve(ctx, videoID, false)
	if err != nil {
		log.Printf("[audio] Resolve failed: %v", err)
		c.emit(Event{State: StateError, VideoID: videoID, Err: err.Error()})
		return err
	}

	log.Printf("[audio] Resolved URL (live=%v): %s. Handing over to native player...", track.IsLive, track.URL)
	c.activeURL = track.URL
	c.curIsLive = track.IsLive
	if err := c.player.Play(track.URL); err != nil {
		log.Printf("[audio] Player.Play failed: %v", err)
		c.emit(Event{State: StateError, VideoID: videoID, Err: err.Error()})
		return err
	}

	log.Printf("[audio] Player.Play successfully initialized pipeline")
	return nil
}

func (c *Controller) Pause() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.player.Pause()
}

func (c *Controller) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.activeVideoID = ""
	c.currentState = StateIdle
	return c.player.Stop()
}

func (c *Controller) SetVolume(v float64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.player.SetVolume(v)
}

func (c *Controller) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.player != nil {
		return c.player.Close()
	}
	return nil
}
