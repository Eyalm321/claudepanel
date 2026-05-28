//go:build !windows && !(darwin && cgo) && !(linux && cgo)
package audio

func New(emit func(Event)) (Player, error) {
	return nil, ErrUnsupported
}
