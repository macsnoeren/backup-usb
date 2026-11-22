//go:build windows

package volume

import (
	"context"
	"errors"
)

var errUnsupported = errors.New("volume detection is not supported on this platform")

type windowsFinder struct{}

// NewPlatformFinder returns the finder for this platform.
func NewPlatformFinder() Finder { return windowsFinder{} }

func (windowsFinder) Volumes(context.Context) ([]Volume, error) { return nil, errUnsupported }
func (windowsFinder) Space(context.Context, Volume) (uint64, uint64, error) {
	return 0, 0, errUnsupported
}
func (windowsFinder) Health(context.Context, Volume) Health { return Unknown }
func (windowsFinder) Mount(_ context.Context, v Volume) (Volume, func() error, error) {
	return v, func() error { return nil }, errUnsupported
}
func (windowsFinder) Eject(context.Context, Volume) error { return errUnsupported }
