//go:build !linux && !windows

package secmem

import "errors"

func lock([]byte) error   { return errors.New("memory locking not supported") }
func unlock([]byte) error { return nil }

// ProtectProcess is not supported on this platform.
func ProtectProcess() error { return nil }

func dumpable() (bool, error) { return false, nil }
