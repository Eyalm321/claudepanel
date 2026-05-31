// Package radio resolves YouTube videos to playable stream URLs.
//
// Livestreams resolve to an HLS manifest URL (played via a top-level <audio>
// element / native player instead of the YouTube IFrame embed — the latter
// silently stays muted in macOS WKWebView's cross-origin iframe even with the
// autoplay grant). Regular VOD videos resolve to a deciphered, audio-only
// direct googlevideo URL. Playlists expand to an ordered list of video IDs.
package radio

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/kkdai/youtube/v2"
)

// YouTube signs stream URLs with ~6h expiry; refresh well before. Playlist
// membership changes far less often, so it gets a much longer TTL.
const (
	cacheTTL         = 1 * time.Hour
	playlistCacheTTL = 6 * time.Hour
)

// ResolvedTrack is a playable stream URL plus whether it is a livestream.
// Livestreams (IsLive) never end on their own — the station player must not
// expect a StateEnded for them.
type ResolvedTrack struct {
	URL    string
	IsLive bool
}

type trackEntry struct {
	track ResolvedTrack
	at    time.Time
}

type playlistEntry struct {
	ids []string
	at  time.Time
}

type Resolver struct {
	client        youtube.Client
	mu            sync.Mutex
	cache         map[string]trackEntry
	playlistCache map[string]playlistEntry
	port          int
}

func New() *Resolver {
	r := &Resolver{
		cache:         map[string]trackEntry{},
		playlistCache: map[string]playlistEntry{},
	}

	// Start local proxy server to stream YouTube VODs safely (bypasses 403 Forbidden).
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err == nil {
		r.port = listener.Addr().(*net.TCPAddr).Port
		go func() {
			_ = http.Serve(listener, r)
		}()
	}

	return r
}

func (r *Resolver) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	videoID := req.URL.Query().Get("id")
	log.Printf("[Proxy] Incoming request for video ID: %s, Range: %q", videoID, req.Header.Get("Range"))
	if videoID == "" {
		http.Error(w, "missing video id", http.StatusBadRequest)
		return
	}

	// Resolve the direct googlevideo URL (cached or fresh)
	track, err := r.resolveDirect(req.Context(), videoID, false)
	if err != nil {
		log.Printf("[Proxy] resolveDirect failed: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Create request to the direct URL
	proxyReq, err := http.NewRequestWithContext(req.Context(), "GET", track.URL, nil)
	if err != nil {
		log.Printf("[Proxy] NewRequestWithContext failed: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Copy incoming Range headers to support seeking/buffering
	if rangeHeader := req.Header.Get("Range"); rangeHeader != "" {
		proxyReq.Header.Set("Range", rangeHeader)
	}

	// Perform the request to Google Video CDN
	resp, err := http.DefaultClient.Do(proxyReq)
	if err != nil {
		log.Printf("[Proxy] Direct GET failed: %v", err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	log.Printf("[Proxy] googlevideo responded with status: %d, Content-Length: %s, Content-Range: %q",
		resp.StatusCode, resp.Header.Get("Content-Length"), resp.Header.Get("Content-Range"))

	// Copy relevant headers back to the player client
	if contentType := resp.Header.Get("Content-Type"); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	if contentLength := resp.Header.Get("Content-Length"); contentLength != "" {
		w.Header().Set("Content-Length", contentLength)
	}
	if contentRange := resp.Header.Get("Content-Range"); contentRange != "" {
		w.Header().Set("Content-Range", contentRange)
	}
	w.Header().Set("Accept-Ranges", "bytes")

	w.WriteHeader(resp.StatusCode)

	n, copyErr := io.Copy(w, resp.Body)
	if copyErr != nil {
		log.Printf("[Proxy] Copy error (sent %d bytes): %v", n, copyErr)
	} else {
		log.Printf("[Proxy] Successfully proxied %d bytes", n)
	}
}

// resolveDirect resolves the actual underlying stream URL (either HLS manifest or direct googlevideo URL).
func (r *Resolver) resolveDirect(ctx context.Context, videoID string, forceRefresh bool) (ResolvedTrack, error) {
	videoID = strings.TrimSpace(videoID)
	if videoID == "" {
		return ResolvedTrack{}, fmt.Errorf("radio: empty video id")
	}
	r.mu.Lock()
	if !forceRefresh {
		if entry, ok := r.cache[videoID]; ok && entry.track.URL != "" && time.Since(entry.at) < cacheTTL {
			r.mu.Unlock()
			return entry.track, nil
		}
	}
	r.mu.Unlock()

	video, err := r.client.GetVideoContext(ctx, videoID)
	if err != nil {
		return ResolvedTrack{}, fmt.Errorf("youtube: get video info for %s: %w", videoID, err)
	}

	// Livestream: HLS manifest is the playable URL and never expires by EOS.
	if video.HLSManifestURL != "" {
		track := ResolvedTrack{URL: video.HLSManifestURL, IsLive: true}
		r.mu.Lock()
		r.cache[videoID] = trackEntry{track: track, at: time.Now()}
		r.mu.Unlock()
		return track, nil
	}

	// VOD: pick an audio-only format, preferring audio/mp4 (itag 140 AAC).
	format := pickAudioFormat(video.Formats)
	if format == nil {
		return ResolvedTrack{}, fmt.Errorf("youtube: no playable format for %s", videoID)
	}
	url, err := r.client.GetStreamURLContext(ctx, video, format)
	if err != nil {
		return ResolvedTrack{}, fmt.Errorf("youtube: stream url for %s: %w", videoID, err)
	}
	track := ResolvedTrack{URL: url, IsLive: false}
	r.mu.Lock()
	r.cache[videoID] = trackEntry{track: track, at: time.Now()}
	r.mu.Unlock()
	return track, nil
}

// Resolve returns the player-facing URL. For livestreams, this is the direct HLS manifest;
// for VODs, this is our local proxy URL that forwards requests to avoid 403 Forbidden.
func (r *Resolver) Resolve(ctx context.Context, videoID string, forceRefresh bool) (ResolvedTrack, error) {
	// First resolve the direct track info so we know if it's a livestream
	track, err := r.resolveDirect(ctx, videoID, forceRefresh)
	if err != nil {
		return ResolvedTrack{}, err
	}

	// If it's a livestream or proxy port is not set, play it directly
	if track.IsLive || r.port == 0 {
		return track, nil
	}

	// For VODs, route through our local HTTP proxy
	proxyURL := fmt.Sprintf("http://127.0.0.1:%d/stream?id=%s", r.port, url.QueryEscape(videoID))
	return ResolvedTrack{
		URL:    proxyURL,
		IsLive: false,
	}, nil
}

// pickAudioFormat selects the best audio-only format: audio/mp4 (itag 140 AAC)
// first as it plays in all three native backends; then any format that carries
// audio channels; then any format at all as a last resort.
func pickAudioFormat(formats youtube.FormatList) *youtube.Format {
	audio := formats.WithAudioChannels()
	if mp4 := audio.Type("audio/mp4"); len(mp4) > 0 {
		mp4.Sort()
		return &mp4[0]
	}
	if len(audio) > 0 {
		audio.Sort()
		return &audio[0]
	}
	if len(formats) > 0 {
		return &formats[0]
	}
	return nil
}

// ExpandPlaylist returns the ordered list of video IDs in the given playlist.
// Results are cached for playlistCacheTTL; pass forceRefresh=true to skip it.
func (r *Resolver) ExpandPlaylist(ctx context.Context, playlistID string, forceRefresh bool) ([]string, error) {
	playlistID = strings.TrimSpace(playlistID)
	if playlistID == "" {
		return nil, fmt.Errorf("radio: empty playlist id")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if !forceRefresh {
		if entry, ok := r.playlistCache[playlistID]; ok && len(entry.ids) > 0 && time.Since(entry.at) < playlistCacheTTL {
			return append([]string(nil), entry.ids...), nil
		}
	}

	pl, err := r.client.GetPlaylistContext(ctx, playlistID)
	if err != nil {
		return nil, fmt.Errorf("youtube: get playlist %s: %w", playlistID, err)
	}
	ids := make([]string, 0, len(pl.Videos))
	for _, v := range pl.Videos {
		if v != nil && v.ID != "" {
			ids = append(ids, v.ID)
		}
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("youtube: playlist %s has no playable videos", playlistID)
	}
	r.playlistCache[playlistID] = playlistEntry{ids: ids, at: time.Now()}
	return append([]string(nil), ids...), nil
}
