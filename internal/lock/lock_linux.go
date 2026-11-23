package lock

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// DefaultPath returns the lock file. Root uses /run/usb-backup, a folder
// only root can write, so no other user can hold or replace the lock;
// other users use their own runtime folder.
func DefaultPath() string {
	if os.Geteuid() == 0 {
		return "/run/usb-backup/usb-backup.lock"
	}
	if d := os.Getenv("XDG_RUNTIME_DIR"); d != "" {
		return filepath.Join(d, "usb-backup.lock")
	}
	if d, err := os.UserCacheDir(); err == nil {
		return filepath.Join(d, "usb-backup", "usb-backup.lock")
	}
	return filepath.Join(os.TempDir(), fmt.Sprintf("usb-backup-%d.lock", os.Getuid()))
}

// Acquire takes the lock without waiting. path "" uses DefaultPath. The
// returned function releases it.
func Acquire(path string) (func(), error) {
	if path == "" {
		path = DefaultPath()
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("lock folder: %w", err)
	}
	if st, err := os.Lstat(dir); err != nil || !st.IsDir() {
		return nil, errors.New("lock folder is not a folder")
	} else if sys, ok := st.Sys().(*syscall.Stat_t); ok && int(sys.Uid) != os.Geteuid() && sys.Uid != 0 {
		return nil, errors.New("lock folder belongs to another user")
	}
	// O_NONBLOCK: a FIFO planted at the path cannot make the open hang;
	// O_NOFOLLOW: a symlink is refused.
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}
	st, err := f.Stat()
	sys, ok := st.Sys().(*syscall.Stat_t)
	if err != nil || !st.Mode().IsRegular() || !ok || int(sys.Uid) != os.Geteuid() {
		_ = f.Close()
		return nil, errors.New("lock file is not a regular file of this user")
	}
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
