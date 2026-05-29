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
}

func New() *Resolver {
	return &Resolver{
		cache:         map[string]trackEntry{},
		playlistCache: map[string]playlistEntry{},
	}
}

// Resolve returns a playable stream URL for the given YouTube video ID. If the
// video is a live broadcast it returns the HLS manifest URL (IsLive=true);
// otherwise it returns a deciphered, audio-only direct URL (preferring
// audio/mp4 / itag 140 AAC, which plays in all three native backends).
//
// Results are cached for cacheTTL per video ID; pass forceRefresh=true to skip
// the cache (used when the player reports an error that may indicate a stale
// signed URL).
func (r *Resolver) Resolve(ctx context.Context, videoID string, forceRefresh bool) (ResolvedTrack, error) {
	videoID = strings.TrimSpace(videoID)
	if videoID == "" {
		return ResolvedTrack{}, fmt.Errorf("radio: empty video id")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if !forceRefresh {
		if entry, ok := r.cache[videoID]; ok && entry.track.URL != "" && time.Since(entry.at) < cacheTTL {
			return entry.track, nil
		}
	}

	video, err := r.client.GetVideoContext(ctx, videoID)
	if err != nil {
		return ResolvedTrack{}, fmt.Errorf("youtube: get video info for %s: %w", videoID, err)
	}

	// Livestream: HLS manifest is the playable URL and never expires by EOS.
	if video.HLSManifestURL != "" {
		track := ResolvedTrack{URL: video.HLSManifestURL, IsLive: true}
		r.cache[videoID] = trackEntry{track: track, at: time.Now()}
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
	r.cache[videoID] = trackEntry{track: track, at: time.Now()}
	return track, nil
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
