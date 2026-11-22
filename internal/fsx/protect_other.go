//go:build !windows

package fsx

import "os"

// Protect marks a file inside root read-only.
func Protect(root *os.Root, rel string) error { return root.Chmod(rel, 0o444) }

// Unprotect makes a protected file writable again.
func Unprotect(root *os.Root, rel string) error { return root.Chmod(rel, 0o644) }
