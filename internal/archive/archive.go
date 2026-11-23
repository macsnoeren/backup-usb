// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Maurice Snoeren
//
// This file is part of USB Backup.
//
// USB Backup is free software: you can redistribute it and/or modify it under
// the terms of the GNU General Public License as published by the Free Software
// Foundation, either version 3 of the License, or (at your option) any later
// version.
//
// USB Backup is distributed in the hope that it will be useful, but WITHOUT ANY
// WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A
// PARTICULAR PURPOSE. See the GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License along with
// USB Backup. If not, see <https://www.gnu.org/licenses/>.

package archive

import (
	"archive/zip"
	"bufio"
	"compress/flate"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/macsnoeren/backup-usb/internal/fsx"
)

// ManifestName is the last entry of every zip: a sha256sum-compatible list
// of all files.
const ManifestName = "_backup/manifest.sha256"

// Locked file handling.
const (
	LockedSkip = "skip"
	LockedFail = "fail"
)

// Options control how a source folder is archived.
type Options struct {
	// CompressionLevel is the Deflate level 1-9; 0 stores everything.
	CompressionLevel int
	// StoreExtensions are stored without compression (case-insensitive,
	// without dot).
	StoreExtensions []string
	// Exclude are patterns (path.Match) checked against the base name and,
	// if they contain "/", against the relative path.
	Exclude []string
	// LockedFiles is "skip" (default) or "fail" for files that cannot be
	// opened.
	LockedFiles string
	// FollowSymlinks archives the target of a symlink to a file instead of
	// the link. Links to folders are always stored as links, so loops are
	// impossible.
	FollowSymlinks bool
	// Progress is called after each file with the bytes read so far.
	Progress func(doneBytes uint64, file string)
	// ManifestTime is the modification time of the manifest entry; zero
	// means now.
	ManifestTime time.Time
	// StoredEnd, if set, is called for every stored (uncompressed) entry
	// once all its data has been written to w, before anything that
	// follows it. It tells a streaming verifier where the data ends, which
	// a zip with data descriptors does not say.
	StoredEnd func(dataOffset, end int64)
}

// Entry describes one entry of the zip. The list of entries is the index.
type Entry struct {
	Path           string    `json:"path"`
	Dir            bool      `json:"dir,omitempty"`
	Link           string    `json:"link,omitempty"`
	Size           uint64    `json:"size"`
	Modified       time.Time `json:"modified"`
	Mode           uint32    `json:"mode,omitempty"`
	UID            *uint32   `json:"uid,omitempty"`
	GID            *uint32   `json:"gid,omitempty"`
	SHA256         string    `json:"sha256,omitempty"`
	CRC32          uint32    `json:"crc32"`
	Method         uint16    `json:"method"`
	HeaderOffset   int64     `json:"headerOffset"`
	DataOffset     int64     `json:"dataOffset"`
	CompressedSize uint64    `json:"compressedSize"`
}

// Skipped is a file that was not archived.
type Skipped struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

// Result describes a written zip.
type Result struct {
	Entries []Entry   `json:"entries"`
	Files   int       `json:"files"`
	Dirs    int       `json:"dirs"`
	Bytes   uint64    `json:"bytes"`
	ZipSize int64     `json:"zipSize"`
	Skipped []Skipped `json:"skipped,omitempty"`
	// Warnings are files that were archived but need attention, such as a
	// file that changed while it was read or a name Windows cannot restore.
	Warnings []Skipped `json:"warnings,omitempty"`
	Manifest []byte    `json:"-"`
	// ManifestEntry is the zip entry of the manifest (not in Entries).
	ManifestEntry Entry `json:"-"`
	// StreamSHA256 is the SHA-256 of all zip bytes as they were written.
	StreamSHA256 string `json:"-"`
}

// AllEntries returns Entries followed by the manifest entry, in zip order.
func (r *Result) AllEntries() []Entry {
	return append(append([]Entry(nil), r.Entries...), r.ManifestEntry)
}

// ErrReadFailed wraps errors reading a source file after it was opened.
var ErrReadFailed = errors.New("read error in source file")

// ErrLocked wraps errors for files that cannot be opened when LockedFiles is
// "fail".
var ErrLocked = errors.New("source file cannot be opened")

type countWriter struct {
	w io.Writer
	n int64
}

func (c *countWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	return n, err
}

// Write archives the folder src as a zip stream to w. Paths in the zip use
// "/" and UTF-8; empty folders are kept as folder entries; modification
// times and (on Unix) permissions are kept; symlinks are stored as links
// unless FollowSymlinks is set. The SHA-256 of every file is computed over
// exactly the bytes that go into the zip, and the manifest is the last
// entry.
func Write(ctx context.Context, src string, w io.Writer, opts Options) (*Result, error) {
	st, err := os.Stat(src)
	if err != nil {
		return nil, fmt.Errorf("source folder: %w", err)
	}
	if !st.IsDir() {
		return nil, fmt.Errorf("source %s is not a folder", src)
	}
	streamHash := sha256.New()
	cw := &countWriter{w: io.MultiWriter(w, streamHash)}
	zw := zip.NewWriter(cw)
	level := opts.CompressionLevel
	if level < 1 || level > 9 {
		level = flate.BestSpeed
	}
	var pool []*flate.Writer
	zw.RegisterCompressor(zip.Deflate, func(out io.Writer) (io.WriteCloser, error) {
		if n := len(pool); n > 0 {
			fw := pool[n-1]
			pool = pool[:n-1]
			fw.Reset(out)
			return &pooledFlate{fw, &pool}, nil
		}
		fw, err := flate.NewWriter(out, level)
		if err != nil {
			return nil, err
		}
		return &pooledFlate{fw, &pool}, nil
	})
	store := map[string]bool{}
	for _, e := range opts.StoreExtensions {
		store[strings.ToLower(strings.TrimPrefix(e, "."))] = true
	}

	res := &Result{}
	var headers []*zip.FileHeader
	var done uint64
	buf := make([]byte, 256<<10)

	addHeader := func(fh *zip.FileHeader, e Entry) (io.Writer, error) {
		before := cw.n
		if e.UID != nil && e.GID != nil {
			fh.Extra = append(fh.Extra, unixOwnerExtra(*e.UID, *e.GID)...)
		}
		fw, err := zw.CreateHeader(fh)
		if err != nil {
			return nil, fmt.Errorf("zip header %s: %w", fh.Name, err)
		}
		if err := zw.Flush(); err != nil {
			return nil, fmt.Errorf("write zip: %w", err)
		}
		// The previous file's data descriptor may have been flushed
		// together with this header; the local header is the last thing
		// written.
		local := int64(30 + len(fh.Name) + len(fh.Extra))
		e.DataOffset = cw.n
		e.HeaderOffset = cw.n - local
		if e.HeaderOffset < before {
			return nil, fmt.Errorf("zip offset bookkeeping failed for %s", fh.Name)
		}
		headers = append(headers, fh)
		res.Entries = append(res.Entries, e)
		return fw, nil
	}

	walkErr := walk(src, opts, func(rel string, info fs.FileInfo, full string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if rel == ManifestName {
			res.Skipped = append(res.Skipped, Skipped{Path: rel, Reason: "reserved name"})
			return nil
		}
		if _, err := fsx.SafeRel(rel); err != nil {
			res.Skipped = append(res.Skipped, Skipped{Path: rel, Reason: "name cannot be restored"})
			return nil
		}
		if _, err := fsx.SafeRelOS(rel, "windows"); err != nil {
			res.Warnings = append(res.Warnings, Skipped{Path: rel, Reason: "name cannot be restored on Windows"})
		}
		mode := info.Mode()
		switch {
		case mode.IsDir():
			fh := &zip.FileHeader{Name: rel + "/", Method: zip.Store, Modified: info.ModTime()}
			fh.SetMode(mode)
			_, err := addHeader(fh, withOwner(Entry{Path: rel + "/", Dir: true, Modified: info.ModTime(), Mode: uint32(mode.Perm())}, info))
			if err == nil {
				res.Dirs++
			}
			return err
		case mode&fs.ModeSymlink != 0:
			target, err := os.Readlink(full)
			if err != nil {
				res.Skipped = append(res.Skipped, Skipped{Path: rel, Reason: "cannot read link"})
				return nil
			}
			fh := &zip.FileHeader{Name: rel, Method: zip.Store, Modified: info.ModTime()}
			fh.SetMode(mode)
			fw, err := addHeader(fh, withOwner(Entry{Path: rel, Link: target, Size: uint64(len(target)), Modified: info.ModTime(), Mode: uint32(mode.Perm())}, info))
			if err != nil {
				return err
			}
			sum := sha256.Sum256([]byte(target))
			res.Entries[len(res.Entries)-1].SHA256 = hex.EncodeToString(sum[:])
			if _, err := io.WriteString(fw, target); err != nil {
				return fmt.Errorf("write zip: %w", err)
			}
			if err := announce(zw, cw, opts, res.Entries[len(res.Entries)-1].DataOffset); err != nil {
				return err
			}
			res.Files++
			return nil
		case !mode.IsRegular():
			res.Skipped = append(res.Skipped, Skipped{Path: rel, Reason: "not a regular file"})
			return nil
		}

		f, err := os.Open(full)
		if err != nil {
			if opts.LockedFiles == LockedFail {
				return fmt.Errorf("%w: %s: %w", ErrLocked, rel, err)
			}
			res.Skipped = append(res.Skipped, Skipped{Path: rel, Reason: "in use or not readable"})
			return nil
		}
		defer func() { _ = f.Close() }() // read-only
		// The file may have been swapped after it was listed (for example
		// for a link to a device or a pipe): check the open handle.
		before, err := f.Stat()
		if err != nil || !before.Mode().IsRegular() || !os.SameFile(before, info) {
			res.Skipped = append(res.Skipped, Skipped{Path: rel, Reason: "changed while the backup ran"})
			return nil
		}
		method := zip.Deflate
		if opts.CompressionLevel == 0 || store[strings.ToLower(strings.TrimPrefix(path.Ext(rel), "."))] {
			method = zip.Store
		}
		fh := &zip.FileHeader{Name: rel, Method: method, Modified: info.ModTime()}
		fh.SetMode(mode)
		fw, err := addHeader(fh, withOwner(Entry{Path: rel, Modified: info.ModTime(), Mode: uint32(mode.Perm())}, info))
		if err != nil {
			return err
		}
		h := sha256.New()
		n, err := copyBuffer(ctx, io.MultiWriter(fw, h), f, buf)
		if err != nil {
			var we writeError
			if errors.As(err, &we) {
				return fmt.Errorf("write zip: %w", we.err)
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("%w: %s: %w", ErrReadFailed, rel, err)
		}
		if method == zip.Store {
			if err := announce(zw, cw, opts, res.Entries[len(res.Entries)-1].DataOffset); err != nil {
				return err
			}
		}
		if after, err := f.Stat(); err != nil || uint64(max(after.Size(), 0)) != n || !after.ModTime().Equal(before.ModTime()) {
			res.Warnings = append(res.Warnings, Skipped{Path: rel, Reason: "changed while it was read"})
		}
		e := &res.Entries[len(res.Entries)-1]
		e.Size = n
		e.SHA256 = hex.EncodeToString(h.Sum(nil))
		res.Files++
		res.Bytes += n
		done += n
		if opts.Progress != nil {
			opts.Progress(done, rel)
		}
		return nil
	}, func(rel, reason string) {
		res.Skipped = append(res.Skipped, Skipped{Path: rel, Reason: reason})
	})
	if walkErr != nil {
		return nil, walkErr
	}

	manifest := Manifest(res.Entries)
	mtime := opts.ManifestTime
	if mtime.IsZero() {
		mtime = time.Now()
	}
	mh := &zip.FileHeader{Name: ManifestName, Method: zip.Deflate, Modified: mtime}
	sum := sha256.Sum256(manifest)
	mw, err := addHeader(mh, Entry{Path: ManifestName, Size: uint64(len(manifest)), Modified: mh.Modified, SHA256: hex.EncodeToString(sum[:])})
	if err != nil {
		return nil, err
	}
	if _, err := mw.Write(manifest); err != nil {
		return nil, fmt.Errorf("write zip: %w", err)
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("finish zip: %w", err)
	}
	for i, fh := range headers {
		e := &res.Entries[i]
		e.CRC32 = fh.CRC32
		e.Method = fh.Method
		e.CompressedSize = fh.CompressedSize64
		if fh.UncompressedSize64 != e.Size {
			return nil, fmt.Errorf("zip size bookkeeping failed for %s", e.Path)
		}
	}
	res.ManifestEntry = res.Entries[len(res.Entries)-1]
	res.Entries = res.Entries[:len(res.Entries)-1]
	res.ZipSize = cw.n
	res.StreamSHA256 = hex.EncodeToString(streamHash.Sum(nil))
	res.Manifest = manifest
	return res, nil
}

// unixOwnerExtra is the Info-ZIP "ux" extra field (0x7875) with the
// numeric owner and group, as zip and unzip on Unix write and read it.
func unixOwnerExtra(uid, gid uint32) []byte {
	b := []byte{0x75, 0x78, 11, 0, 1, 4, 0, 0, 0, 0, 4, 0, 0, 0, 0}
	binary.LittleEndian.PutUint32(b[6:], uid)
	binary.LittleEndian.PutUint32(b[11:], gid)
	return b
}

// withOwner fills the owner of an entry from the file info.
func withOwner(e Entry, info fs.FileInfo) Entry {
	if uid, gid, ok := ownerOf(info); ok {
		e.UID, e.GID = &uid, &gid
	}
	return e
}

// announce flushes a stored entry's data and reports where it ends.
func announce(zw *zip.Writer, cw *countWriter, opts Options, dataOffset int64) error {
	if opts.StoredEnd == nil {
		return nil
	}
	if err := zw.Flush(); err != nil {
		return fmt.Errorf("write zip: %w", err)
	}
	opts.StoredEnd(dataOffset, cw.n)
	return nil
}

// pooledFlate returns its flate writer to the pool on Close, so archives
// with many small files do not allocate a compressor per file.
type pooledFlate struct {
	*flate.Writer
	pool *[]*flate.Writer
}

func (p *pooledFlate) Close() error {
	err := p.Writer.Close()
	*p.pool = append(*p.pool, p.Writer)
	return err
}

type writeError struct{ err error }

func (e writeError) Error() string { return e.err.Error() }

// copyBuffer copies src to dst, checking ctx between chunks. Write errors
// are wrapped in writeError so callers can tell them from read errors.
func copyBuffer(ctx context.Context, dst io.Writer, src io.Reader, buf []byte) (uint64, error) {
	var n uint64
	for {
		if err := ctx.Err(); err != nil {
			return n, err
		}
		r, rerr := src.Read(buf)
		if r > 0 {
			if _, werr := dst.Write(buf[:r]); werr != nil {
				return n, writeError{werr}
			}
			n += uint64(r)
		}
		if rerr == io.EOF {
			return n, nil
		}
		if rerr != nil {
			return n, rerr
		}
	}
}

// Manifest returns the sha256sum-compatible manifest for the file entries.
func Manifest(entries []Entry) []byte {
	var b strings.Builder
	for _, e := range entries {
		if e.Dir {
			continue
		}
		b.WriteString(ManifestLine(e.SHA256, e.Path))
	}
	return []byte(b.String())
}

// ManifestLine formats one sha256sum line. Names with a backslash or a
// newline are escaped the way GNU sha256sum does.
func ManifestLine(sum, name string) string {
	if strings.ContainsAny(name, "\\\n\r") {
		name = strings.NewReplacer(`\`, `\\`, "\n", `\n`, "\r", `\r`).Replace(name)
		return `\` + sum + "  " + name + "\n"
	}
	return sum + "  " + name + "\n"
}

// MaxManifestSize bounds manifests read back from a disk.
const MaxManifestSize = 256 << 20

// ParseManifest parses a sha256sum file into name -> hex sum.
func ParseManifest(r io.Reader) (map[string]string, error) {
	out := map[string]string{}
	sc := bufio.NewScanner(io.LimitReader(r, MaxManifestSize))
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			continue
		}
		escaped := strings.HasPrefix(line, `\`)
		line = strings.TrimPrefix(line, `\`)
		sum, name, ok := strings.Cut(line, "  ")
		if !ok || len(sum) != 64 {
			return nil, fmt.Errorf("invalid manifest line")
		}
		if _, err := hex.DecodeString(sum); err != nil {
			return nil, fmt.Errorf("invalid manifest line")
		}
		if escaped {
			name = unescape(name)
		}
		if _, dup := out[name]; dup {
			return nil, fmt.Errorf("duplicate manifest entry %q", name)
		}
		out[name] = strings.ToLower(sum)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	return out, nil
}

func unescape(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
			switch s[i] {
			case 'n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			default:
				b.WriteByte(s[i])
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// walk visits every entry under src in sorted order, applying excludes.
// rel is the slash path relative to src. Folders and entries that cannot be
// read are passed to skip (or fail the walk with lockedFiles: fail).
func walk(src string, opts Options, fn func(rel string, info fs.FileInfo, full string) error, skip func(rel, reason string)) error {
	var visit func(dir, rel string, depth int) error
	visit = func(dir, rel string, depth int) error {
		if depth > 256 {
			return fmt.Errorf("folder nesting deeper than 256 levels at %s", rel)
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			if rel == "" {
				return fmt.Errorf("read source folder: %w", err)
			}
			if opts.LockedFiles == LockedFail {
				return fmt.Errorf("%w: %s: %w", ErrLocked, rel, err)
			}
			// Entries read before the error are still archived.
			skip(rel+"/", "folder cannot be read completely")
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		for _, de := range entries {
			name := de.Name()
			r := name
			if rel != "" {
				r = rel + "/" + name
			}
			if excluded(opts.Exclude, name, r) {
				continue
			}
			full := filepath.Join(dir, name)
			info, err := os.Lstat(full)
			if err != nil {
				if opts.LockedFiles == LockedFail {
					return fmt.Errorf("%w: %s: %w", ErrLocked, r, err)
				}
				skip(r, "cannot be read")
				continue
			}
			if info.Mode()&fs.ModeSymlink != 0 && opts.FollowSymlinks {
				if target, err := os.Stat(full); err == nil && !target.IsDir() {
					info = target
				}
			}
			if err := fn(r, info, full); err != nil {
				return err
			}
			if info.IsDir() {
				if err := visit(full, r, depth+1); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return visit(src, "", 0)
}

func excluded(patterns []string, base, rel string) bool {
	for _, p := range patterns {
		target := base
		if strings.Contains(p, "/") {
			target = rel
		}
		if ok, _ := path.Match(p, target); ok {
			return true
		}
		if ok, _ := path.Match(strings.ToLower(p), strings.ToLower(target)); ok && caseInsensitiveFS {
			return true
		}
	}
	return false
}

// Scan returns the number of files and bytes that Write would archive.
func Scan(src string, opts Options) (files int, bytes uint64, err error) {
	err = walk(src, opts, func(_ string, info fs.FileInfo, _ string) error {
		if info.Mode().IsRegular() {
			files++
			bytes += uint64(max(info.Size(), 0))
		}
		return nil
	}, func(string, string) {})
	return files, bytes, err
}
