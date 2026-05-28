// Package radio resolves the YouTube livestream behind "Claude FM" to a
// playable HLS manifest URL. Used so the frontend can play audio via a
// top-level <audio> element instead of the YouTube IFrame embed — which
// is blocked from unmuting in macOS WKWebView's cross-origin iframe even
// with mediaTypesRequiringUserActionForPlayback disabled.
package radio

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/kkdai/youtube/v2"
)

// VideoID is the YouTube ID of the Lo-Fi livestream we surface as "Claude FM".
const VideoID = "YmQ7jRgf4f0"

// YouTube signs livestream manifest URLs with ~6h expiry; refresh well before.
const cacheTTL = 1 * time.Hour

type Resolver struct {
	client    youtube.Client
	mu        sync.Mutex
	cachedURL string
	cachedAt  time.Time
}

func New() *Resolver { return &Resolver{} }

// StreamURL returns the HLS manifest URL for the livestream. The result is
// cached for cacheTTL; pass forceRefresh=true to skip the cache (use when the
// frontend reports playback errors).
func (r *Resolver) StreamURL(ctx context.Context, forceRefresh bool) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !forceRefresh && r.cachedURL != "" && time.Since(r.cachedAt) < cacheTTL {
		return r.cachedURL, nil
	}
	video, err := r.client.GetVideoContext(ctx, VideoID)
	if err != nil {
		return "", fmt.Errorf("youtube: get video info: %w", err)
	}
	if video.HLSManifestURL == "" {
		return "", fmt.Errorf("youtube: no HLS manifest for %s (livestream offline?)", VideoID)
	}
	r.cachedURL = video.HLSManifestURL
	r.cachedAt = time.Now()
	return r.cachedURL, nil
}
