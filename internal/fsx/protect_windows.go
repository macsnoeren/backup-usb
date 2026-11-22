package fsx

import (
	"os"
	"path/filepath"
	"syscall"
)

// Protect marks a file inside root read-only and hidden.
func Protect(root *os.Root, rel string) error {
	p, err := syscall.UTF16PtrFromString(filepath.Join(root.Name(), filepath.FromSlash(rel)))
	if err != nil {
		return err
	}
	return syscall.SetFileAttributes(p, syscall.FILE_ATTRIBUTE_READONLY|syscall.FILE_ATTRIBUTE_HIDDEN)
}

// Unprotect makes a protected file writable again (needed before it can be
// replaced on Windows).
func Unprotect(root *os.Root, rel string) error {
	p, err := syscall.UTF16PtrFromString(filepath.Join(root.Name(), filepath.FromSlash(rel)))
	if err != nil {
		return err
	}
	return syscall.SetFileAttributes(p, syscall.FILE_ATTRIBUTE_NORMAL)
}
