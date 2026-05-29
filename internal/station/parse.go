// Package station turns user-configured collections of YouTube items into an
// auto-advancing, looping radio queue layered above the single-track
// audio.Controller.
package station

import (
	"fmt"
	"regexp"
	"strings"

	"claudepanel/internal/config"
)

// videoRefRe captures the 11-char video ID from any common YouTube URL form,
// regardless of scheme/host (https://, https://www., www., youtube.com, m./
// music.youtube.com, youtu.be). Checked before list= so watch?v=X&list=Y
// resolves to the single video X (least-surprising) and youtu.be/X?list=Y stays
// a video.
var videoRefRe = regexp.MustCompile(`(?:youtu\.be/|/shorts/|/embed/|/v/|/live/|[?&]v=)([0-9A-Za-z_-]{11})`)

// listRe captures a playlist ID from a list= parameter (playlist?list=,
// watch?list=, …&list=). YouTube playlist IDs are 13+ chars of the URL-safe
// alphabet.
var listRe = regexp.MustCompile(`[?&]list=([0-9A-Za-z_-]{13,})`)

// Bare IDs pasted without any URL ("just the suffix code"). Video IDs are
// exactly 11 chars; playlist IDs are 13+ — the length gap disambiguates them.
var (
	bareVideoRe    = regexp.MustCompile(`^[0-9A-Za-z_-]{11}$`)
	barePlaylistRe = regexp.MustCompile(`^[0-9A-Za-z_-]{13,}$`)
)

// ParseItem classifies a single user input into a StationItem. It accepts every
// common form — with or without scheme/host (https://, https://www., www.,
// youtube.com/…) and bare IDs:
//
//   - watch?v=X[&list=Y]                              → video X (list ignored)
//   - youtu.be/X, /shorts/X, /embed/X, /v/X, /live/X  → video X
//   - playlist?list=Y, watch?list=Y, …&list=Y         → playlist Y
//   - bare 11-char ID                                 → video
//   - bare 13+-char ID                                → playlist
//
// Live-vs-VOD is deferred to the resolver, so a watch/live URL is always "video"
// here; the engine treats video and livestream identically (one ID).
func ParseItem(input string) (config.StationItem, error) {
	raw := strings.TrimSpace(input)
	if raw == "" {
		return config.StationItem{}, fmt.Errorf("empty input")
	}

	if m := videoRefRe.FindStringSubmatch(raw); m != nil {
		return config.StationItem{Kind: config.ItemVideo, ID: m[1], Raw: raw}, nil
	}
	if m := listRe.FindStringSubmatch(raw); m != nil {
		return config.StationItem{Kind: config.ItemPlaylist, ID: m[1], Raw: raw}, nil
	}
	if bareVideoRe.MatchString(raw) {
		return config.StationItem{Kind: config.ItemVideo, ID: raw, Raw: raw}, nil
	}
	if barePlaylistRe.MatchString(raw) {
		return config.StationItem{Kind: config.ItemPlaylist, ID: raw, Raw: raw}, nil
	}
	return config.StationItem{}, fmt.Errorf("unrecognized YouTube URL or ID: %q", raw)
}
