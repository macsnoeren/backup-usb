// Package fsx has small file system helpers: write to a .part file, sync,
// rename, and safe joining of untrusted relative paths.
package fsx

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unicode"
)

// PartSuffix is the suffix of files that are still being written.
const PartSuffix = ".part"

// WriteFileAtomic writes data to path+".part", syncs it and renames it to
// path. On error the .part file is removed.
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	part := path + PartSuffix
	f, err := os.OpenFile(part, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return fmt.Errorf("create %s: %w", filepath.Base(part), err)
	}
	_, werr := f.Write(data)
	serr := f.Sync()
	cerr := f.Close()
	if err := errors.Join(werr, serr, cerr); err != nil {
		_ = os.Remove(part)
		return fmt.Errorf("write %s: %w", filepath.Base(part), err)
	}
	if err := Rename(part, path); err != nil {
		_ = os.Remove(part)
		return err
	}
	return nil
}

// Rename renames from to to (replacing to) and syncs the directory.
func Rename(from, to string) error {
	if err := os.Rename(from, to); err != nil {
		return fmt.Errorf("rename %s: %w", filepath.Base(from), err)
	}
	return SyncDir(filepath.Dir(to))
}

// SyncDir flushes directory metadata to disk where the platform supports
// it.
func SyncDir(dir string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	d, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("open directory: %w", err)
	}
	serr := d.Sync()
	cerr := d.Close()
	if serr != nil && !errors.Is(serr, os.ErrInvalid) {
		return fmt.Errorf("sync directory: %w", serr)
	}
	return cerr
}

// SafeRel checks a relative slash path from an untrusted source (a zip entry
// or an index) and returns it cleaned. It rejects absolute paths, "..",
// drive letters, UNC paths, backslashes, NUL and control characters, and
// Windows device names.
func SafeRel(name string) (string, error) {
	if name == "" {
		return "", errors.New("empty path")
	}
	if strings.ContainsRune(name, '\\') || strings.ContainsRune(name, ':') {
		return "", fmt.Errorf("unsafe path %q", name)
	}
	for _, r := range name {
		if r == 0 || unicode.IsControl(r) {
			return "", fmt.Errorf("unsafe path %q", name)
		}
	}
	if strings.HasPrefix(name, "/") {
		return "", fmt.Errorf("absolute path %q", name)
	}
	trimmed := strings.TrimSuffix(name, "/")
	for _, comp := range strings.Split(trimmed, "/") {
		if comp == "" || comp == "." || comp == ".." {
			return "", fmt.Errorf("unsafe path %q", name)
		}
		if isWindowsDevice(comp) {
			return "", fmt.Errorf("reserved name in path %q", name)
		}
	}
	return trimmed, nil
}

// SafeJoin joins an untrusted relative slash path to base and guarantees
// that the result stays inside base.
func SafeJoin(base, name string) (string, error) {
	rel, err := SafeRel(name)
	if err != nil {
		return "", err
	}
	p := filepath.Join(base, filepath.FromSlash(rel))
	r, err := filepath.Rel(base, p)
	if err != nil || r == ".." || strings.HasPrefix(r, ".."+string(filepath.Separator)) || filepath.IsAbs(r) {
		return "", fmt.Errorf("path %q escapes the target folder", name)
	}
	return p, nil
}

func isWindowsDevice(comp string) bool {
	base := strings.ToUpper(comp)
	if i := strings.IndexByte(base, '.'); i >= 0 {
		base = base[:i]
	}
	base = strings.TrimRight(base, " ")
	switch base {
	case "CON", "PRN", "AUX", "NUL", "CONIN$", "CONOUT$":
		return true
	}
	if len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '0' && base[3] <= '9' {
		return true
	}
	return false
}
