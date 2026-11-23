//go:build !linux && !windows

package lock

// Acquire is a no-op on other platforms.
func Acquire(string) (func(), error) { return func() {}, nil }

// DefaultPath returns "".
func DefaultPath() string { return "-" }
