package lock

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// DefaultPath is the lock file.
const DefaultPath = "/run/lock/usb-backup.lock"

// Acquire takes the lock without waiting. path "" uses DefaultPath, or a
// per-user file when /run/lock cannot be used. The returned function
// releases it.
func Acquire(path string) (func(), error) {
	if path == "" {
		path = DefaultPath
		if f, err := openLockFile(path); err == nil {
			return lockFile(f)
		}
		dir := os.Getenv("XDG_RUNTIME_DIR")
		if dir == "" {
			dir = os.TempDir()
		}
		path = filepath.Join(dir, fmt.Sprintf("usb-backup-%d.lock", os.Getuid()))
	}
	f, err := openLockFile(path)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}
	return lockFile(f)
}

// openLockFile opens or creates the lock file. A file created by another
// user (root) can still be locked through a read-only descriptor.
func openLockFile(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|syscall.O_NOFOLLOW, 0o644) //nolint:gosec // lock file holds no data
	if err != nil {
		f, err = os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	}
	return f, err
}

func lockFile(f *os.File) (func(), error) {
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, ErrBusy
		}
		return nil, fmt.Errorf("lock: %w", err)
	}
	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	}, nil
}
