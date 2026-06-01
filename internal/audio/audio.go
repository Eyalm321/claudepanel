package audio

import "errors"

type State string

const (
	StateIdle    State = "idle"
	StateLoading State = "loading"
	StatePlaying State = "playing"
	StatePaused  State = "paused"
	StateError   State = "error"
	// StateEnded means the current track played to its natural end (EOS).
	// Distinct from idle/paused so the station player can auto-advance.
	// Livestreams (HLS) never emit it.
	StateEnded State = "ended"
)

type Event struct {
	State   State  `json:"state"`
	VideoID string `json:"videoID,omitempty"`
	Err     string `json:"error,omitempty"`
	// StationIdx is stamped by the station player on events it forwards to the
	// frontend, so the UI can filter to the active station. The audio layer
	// itself leaves it at 0.
	StationIdx int `json:"stationIdx"`
}

type Player interface {
	Play(url string) error
	// Resume continues the currently-loaded track from its paused position
	// (distinct from Play, which loads a source and starts from the beginning).
	Resume() error
	Pause() error
	Stop() error
	SetVolume(v float64) error // 0..1
	Close() error
}

var ErrUnsupported = errors.New("audio: unsupported platform")
