//go:build !windows && !linux

package volume

import (
	"context"
	"errors"
)

var errUnsupported = errors.New("volume detection is not supported on this platform")

type stubFinder struct{}

// NewPlatformFinder returns the finder for this platform.
func NewPlatformFinder() Finder { return stubFinder{} }

func (stubFinder) Volumes(context.Context) ([]Volume, error) { return nil, errUnsupported }
func (stubFinder) Space(context.Context, Volume) (uint64, uint64, error) {
	return 0, 0, errUnsupported
}
func (stubFinder) Health(context.Context, Volume) Health { return Unknown }
func (stubFinder) Mount(_ context.Context, v Volume) (Volume, func() error, error) {
	return v, func() error { return nil }, errUnsupported
}
func (stubFinder) Eject(context.Context, Volume) error { return errUnsupported }
