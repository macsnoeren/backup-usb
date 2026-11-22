package fsx

import (
	"errors"
	"fmt"
	"io"
	"os"
)

// ErrNotRegular is returned when a file on a disk is not a plain file (a
// symlink, FIFO, device or folder).
var ErrNotRegular = errors.New("not a regular file")

// OpenRegular opens rel inside root for reading, but only if it is a
// regular file: no symlink, FIFO or device, so a planted special file can
// neither hang the program nor point elsewhere. The check is repeated on
// the open handle.
func OpenRegular(root *os.Root, rel string) (*os.File, error) {
	st, err := root.Lstat(rel)
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() {
		return nil, fmt.Errorf("%s: %w", rel, ErrNotRegular)
	}
	f, err := root.OpenFile(rel, os.O_RDONLY|oNonblock, 0)
	if err != nil {
		return nil, err
	}
	st2, err := f.Stat()
	if err != nil || !st2.Mode().IsRegular() || !os.SameFile(st, st2) {
		_ = f.Close()
		return nil, fmt.Errorf("%s: %w", rel, ErrNotRegular)
	}
	return f, nil
}

// ReadRegular reads a regular file of at most maxSize bytes inside root.
func ReadRegular(root *os.Root, rel string, maxSize int64) ([]byte, error) {
	f, err := OpenRegular(root, rel)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }() // read-only
	data, err := io.ReadAll(io.LimitReader(f, maxSize+1))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", rel, err)
	}
	if int64(len(data)) > maxSize {
		return nil, fmt.Errorf("%s is too large", rel)
	}
	return data, nil
}
