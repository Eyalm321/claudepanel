// Package station turns user-configured collections of YouTube items into an
// auto-advancing, looping radio queue layered above the single-track
// audio.Controller.
package station

import (
	"fmt"
	"regexp"
	"strings"

	"claudepanel/internal/config"

	"github.com/kkdai/youtube/v2"
)

// An explicit video reference in any common YouTube URL form. Captures the
// 11-char video ID. Checked before list= so watch?v=X&list=Y resolves to the
// single video X (least-surprising) and youtu.be/X?list=Y stays a video.
var videoRefRe = regexp.MustCompile(`(?:youtu\.be/|/shorts/|/embed/|/v/|[?&]v=)([0-9A-Za-z_-]{11})`)

// A list= parameter (playlist URLs and playlist?list= forms). YouTube playlist
// IDs are 13–42 chars of the URL-safe alphabet.
var listRe = regexp.MustCompile(`[?&]list=([0-9A-Za-z_-]{13,42})`)

// ParseItem classifies a single user input (URL or bare ID) into a StationItem.
//
//   - watch?v=X&list=Y   → video X (the list is ignored)
//   - youtu.be/X, /shorts/X, /embed/X, watch?v=X → video X
//   - playlist?list=Y or any URL with list= and no video ref → playlist Y
//   - bare 11-char ID (or other youtu.* form) → video
//
// Live-vs-VOD is deferred to the resolver, so a watch URL is always "video"
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

	id, err := youtube.ExtractVideoID(raw)
	if err != nil {
		return config.StationItem{}, fmt.Errorf("unrecognized YouTube URL or ID: %q", raw)
	}
	return config.StationItem{Kind: config.ItemVideo, ID: id, Raw: raw}, nil
}
