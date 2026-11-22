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
	"unicode/utf8"
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
// or an index) for the current platform and returns it cleaned.
func SafeRel(name string) (string, error) { return SafeRelOS(name, runtime.GOOS) }

// SafeRelOS checks name for the given GOOS. Everywhere it rejects empty and
// absolute paths, ".", "..", empty components, NUL and control characters,
// invalid UTF-8 and invisible format characters (such as bidi overrides).
// For windows it also rejects backslashes, drive letters and streams (":"),
// the characters <>"|?*, components ending in a dot or space and device
// names such as CON, NUL, COM1 and LPT¹.
func SafeRelOS(name, goos string) (string, error) {
	if name == "" {
		return "", errors.New("empty path")
	}
	if !utf8.ValidString(name) {
		return "", fmt.Errorf("path %q is not valid UTF-8", name)
	}
	for _, r := range name {
		if r == 0 || unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return "", fmt.Errorf("unsafe path %q", name)
		}
	}
	if strings.HasPrefix(name, "/") {
		return "", fmt.Errorf("absolute path %q", name)
	}
	windows := goos == "windows"
	if windows && strings.ContainsAny(name, `\:<>"|?*`) {
		return "", fmt.Errorf("path %q is not valid on Windows", name)
	}
	trimmed := strings.TrimSuffix(name, "/")
	for _, comp := range strings.Split(trimmed, "/") {
		if comp == "" || comp == "." || comp == ".." {
			return "", fmt.Errorf("unsafe path %q", name)
		}
		if windows && (strings.HasSuffix(comp, ".") || strings.HasSuffix(comp, " ") || isWindowsDevice(comp)) {
			return "", fmt.Errorf("path %q is not valid on Windows", name)
		}
	}
	return trimmed, nil
}

// SafeJoin joins an untrusted relative slash path to base and guarantees
// that the result stays inside base lexically. Callers that create files
// must still go through an os.Root, so symlinks cannot lead outside.
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
	for _, p := range []string{"COM", "LPT"} {
		if rest, ok := strings.CutPrefix(base, p); ok {
			switch rest {
			case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "¹", "²", "³":
				return true
			}
		}
	}
	return false
}
