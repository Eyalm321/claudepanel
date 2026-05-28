// Package radio resolves YouTube livestreams to playable HLS manifest URLs.
// Used so the frontend can play audio via a top-level <audio> element instead
// of the YouTube IFrame embed — the latter silently stays muted in macOS
// WKWebView's cross-origin iframe even with the autoplay grant.
package radio

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/kkdai/youtube/v2"
)

// YouTube signs livestream manifest URLs with ~6h expiry; refresh well before.
const cacheTTL = 1 * time.Hour

type cacheEntry struct {
	url string
	at  time.Time
}

type Resolver struct {
	client youtube.Client
	mu     sync.Mutex
	cache  map[string]cacheEntry
}

func New() *Resolver { return &Resolver{cache: map[string]cacheEntry{}} }

// StreamURL returns the HLS manifest URL for the given YouTube livestream
// video ID. Results are cached for cacheTTL per video ID; pass
// forceRefresh=true to skip the cache (use when the frontend reports playback
// errors that may indicate a stale signed URL).
func (r *Resolver) StreamURL(ctx context.Context, videoID string, forceRefresh bool) (string, error) {
	videoID = strings.TrimSpace(videoID)
	if videoID == "" {
		return "", fmt.Errorf("radio: empty video id")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if !forceRefresh {
		if entry, ok := r.cache[videoID]; ok && entry.url != "" && time.Since(entry.at) < cacheTTL {
			return entry.url, nil
		}
	}
	video, err := r.client.GetVideoContext(ctx, videoID)
	if err != nil {
		return "", fmt.Errorf("youtube: get video info for %s: %w", videoID, err)
	}
	if video.HLSManifestURL == "" {
		return "", fmt.Errorf("youtube: no HLS manifest for %s (livestream offline?)", videoID)
	}
	r.cache[videoID] = cacheEntry{url: video.HLSManifestURL, at: time.Now()}
	return video.HLSManifestURL, nil
}
