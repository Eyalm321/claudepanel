package audio

import "errors"

type State string

const (
	StateIdle    State = "idle"
	StateLoading State = "loading"
	StatePlaying State = "playing"
	StatePaused  State = "paused"
	StateError   State = "error"
)

type Event struct {
	State   State  `json:"state"`
	VideoID string `json:"videoID,omitempty"`
	Err     string `json:"error,omitempty"`
}

type Player interface {
	Play(url string) error
	Pause() error
	Stop() error
	SetVolume(v float64) error // 0..1
	Close() error
}

var ErrUnsupported = errors.New("audio: unsupported platform")
