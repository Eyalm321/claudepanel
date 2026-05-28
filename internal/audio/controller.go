package audio

import (
	"context"
	"log"
	"sync"
)

type StreamResolver interface {
	StreamURL(ctx context.Context, videoID string, forceRefresh bool) (string, error)
}

type Controller struct {
	mu            sync.Mutex
	player        Player
	resolver      StreamResolver
	emit          func(Event)
	activeVideoID string
	activeURL     string
	currentState  State
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

	log.Printf("[audio] Player event received: State=%s, Err=%s", ev.State, ev.Err)

	if ev.State == StateError {
		if !c.retried && c.activeVideoID != "" {
			c.retried = true
			log.Printf("[audio] Player encountered error: %s. Attempting once-off stream URL refresh...", ev.Err)
			
			// Retrying asynchronously
			go func(videoID string) {
				freshURL, err := c.resolver.StreamURL(context.Background(), videoID, true)
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
				c.activeURL = freshURL
				player := c.player
				c.mu.Unlock()

				if player != nil {
					log.Printf("[audio] Replaying refreshed URL: %s", freshURL)
					if err := player.Play(freshURL); err != nil {
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
	c.retried = false
	c.emit(Event{State: StateLoading, VideoID: videoID})

	log.Printf("[audio] Resolving stream URL for video %s...", videoID)
	url, err := c.resolver.StreamURL(ctx, videoID, false)
	if err != nil {
		log.Printf("[audio] StreamURL resolve failed: %v", err)
		c.emit(Event{State: StateError, VideoID: videoID, Err: err.Error()})
		return err
	}

	log.Printf("[audio] Resolved URL: %s. Handing over to native player...", url)
	c.activeURL = url
	if err := c.player.Play(url); err != nil {
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
