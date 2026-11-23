//go:build !linux && !windows

package task

import "errors"

type none struct{}

// NewPlatformRegistrar returns a registrar that refuses.
func NewPlatformRegistrar() Registrar { return none{} }

func (none) Install(Spec) error     { return errors.New("not supported on this platform") }
func (none) Uninstall(Trigger) error { return errors.New("not supported on this platform") }
