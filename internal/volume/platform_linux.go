//go:build linux

package volume

import (
	"context"
	"errors"
)

var errUnsupported = errors.New("volume detection is not supported on this platform")

type linuxFinder struct{}

// NewPlatformFinder returns the finder for this platform.
func NewPlatformFinder() Finder { return linuxFinder{} }

func (linuxFinder) Volumes(context.Context) ([]Volume, error) { return nil, errUnsupported }
func (linuxFinder) Space(context.Context, Volume) (uint64, uint64, error) {
	return 0, 0, errUnsupported
}
func (linuxFinder) Health(context.Context, Volume) Health { return Unknown }
func (linuxFinder) Mount(_ context.Context, v Volume) (Volume, func() error, error) {
	return v, func() error { return nil }, errUnsupported
}
func (linuxFinder) Eject(context.Context, Volume) error { return errUnsupported }
