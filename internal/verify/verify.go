package verify

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/macsnoeren/backup-usb/internal/archive"
)

// ErrFailed wraps every verification failure.
var ErrFailed = errors.New("backup verification failed")

// MaxEntries bounds the number of entries accepted in a zip from a disk.
const MaxEntries = 20_000_000

// Options tune the check.
type Options struct {
	// ExpectedFiles is the number of files the writer put in; -1 if
	// unknown.
	ExpectedFiles int
	// Progress is called with the bytes checked so far.
	Progress func(done uint64)
	// Entries, if set, are the entries the writer recorded. Every local
	// file header is then compared with them, which archive/zip does not
	// do (it reads names from the central directory only).
	Entries []archive.Entry
}

// Report describes a successful check.
type Report struct {
	Files int    `json:"files"`
	Bytes uint64 `json:"bytes"`
}

func fail(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrFailed, fmt.Sprintf(format, args...))
}

// File checks the zip at path.
func File(ctx context.Context, path string, opts Options) (*Report, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open zip: %w", err)
	}
	defer func() { _ = f.Close() }() // read-only
	st, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat zip: %w", err)
	}
	return Zip(ctx, f, st.Size(), opts)
}

// Zip opens the zip, unpacks every entry to io.Discard (archive/zip checks
// the CRC-32), compares every SHA-256 with the manifest and checks that the
// number of files matches exactly. The zip is untrusted input.
func Zip(ctx context.Context, r io.ReaderAt, size int64, opts Options) (*Report, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, fail("cannot open zip: %v", err)
	}
	if len(zr.File) > MaxEntries {
		return nil, fail("zip has %d entries", len(zr.File))
	}
	var manifestFile *zip.File
	seen := map[string]bool{}
	for _, f := range zr.File {
		if seen[f.Name] {
			return nil, fail("duplicate entry %q", f.Name)
		}
		seen[f.Name] = true
		if f.Name == archive.ManifestName {
			manifestFile = f
		}
	}
	if manifestFile == nil {
		return nil, fail("manifest %s missing", archive.ManifestName)
	}
	if manifestFile.UncompressedSize64 > archive.MaxManifestSize {
		return nil, fail("manifest too large")
	}
	mr, err := manifestFile.Open()
	if err != nil {
		return nil, fail("open manifest: %v", err)
	}
	manifest, err := archive.ParseManifest(mr)
	cerr := mr.Close()
	if err != nil {
		return nil, fail("manifest: %v", err)
	}
	if cerr != nil {
		return nil, fail("manifest: %v", cerr)
	}

	if opts.Entries != nil {
		if err := checkLocalHeaders(r, size, zr, opts.Entries); err != nil {
			return nil, err
		}
	}

	rep := &Report{}
	buf := make([]byte, 256<<10)
	for _, f := range zr.File {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if f.Name == archive.ManifestName {
			continue
		}
		if f.FileInfo().IsDir() {
			if f.UncompressedSize64 != 0 {
				return nil, fail("folder entry %q has data", f.Name)
			}
			continue
		}
		want, ok := manifest[f.Name]
		if !ok {
			return nil, fail("%q is not in the manifest", f.Name)
		}
		sum, n, err := hashEntry(ctx, f, buf)
		if err != nil {
			return nil, fail("%q: %v", f.Name, err)
		}
		if sum != want {
			return nil, fail("%q: SHA-256 does not match the manifest", f.Name)
		}
		delete(manifest, f.Name)
		rep.Files++
		rep.Bytes += n
		if opts.Progress != nil {
			opts.Progress(rep.Bytes)
		}
	}
	if len(manifest) > 0 {
		for name := range manifest {
			return nil, fail("%q is in the manifest but missing from the zip", name)
		}
	}
	if opts.ExpectedFiles >= 0 && rep.Files != opts.ExpectedFiles {
		return nil, fail("zip has %d files, expected %d", rep.Files, opts.ExpectedFiles)
	}
	return rep, nil
}

func hashEntry(ctx context.Context, f *zip.File, buf []byte) (string, uint64, error) {
	rc, err := f.Open()
	if err != nil {
		return "", 0, err
	}
	defer func() { _ = rc.Close() }() // read-only
	h := sha256.New()
	// Never read more than the declared size, so a zip bomb cannot run on.
	lr := &io.LimitedReader{R: rc, N: int64(min(f.UncompressedSize64, 1<<62)) + 1}
	var n uint64
	for {
		if err := ctx.Err(); err != nil {
			return "", n, err
		}
		m, rerr := lr.Read(buf)
		h.Write(buf[:m])
		n += uint64(m) //nolint:gosec // Read never returns a negative count
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return "", n, rerr
		}
	}
	if n != f.UncompressedSize64 {
		return "", n, fmt.Errorf("size %d does not match header %d", n, f.UncompressedSize64)
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

// checkLocalHeaders compares every local file header and data descriptor
// with the entries recorded by the writer and with the central directory.
// archive/zip reads names, times and extra fields from the central
// directory only, so damage there would otherwise go unnoticed.
func checkLocalHeaders(r io.ReaderAt, size int64, zr *zip.Reader, entries []archive.Entry) error {
	central := make(map[string]*zip.File, len(zr.File))
	for _, f := range zr.File {
		central[f.Name] = f
	}
	if len(entries) != len(zr.File) {
		return fail("zip has %d entries, the writer recorded %d", len(zr.File), len(entries))
	}
	le16 := func(b []byte) uint16 { return uint16(b[0]) | uint16(b[1])<<8 }
	le32 := func(b []byte) uint32 { return uint32(le16(b)) | uint32(le16(b[2:]))<<16 }
	le64 := func(b []byte) uint64 { return uint64(le32(b)) | uint64(le32(b[4:]))<<32 }
	for i, e := range entries {
		f := central[e.Path]
		if f == nil || zr.File[i] != f {
			return fail("%q missing from the central directory or out of order", e.Path)
		}
		if f.CRC32 != e.CRC32 || f.CompressedSize64 != e.CompressedSize || f.UncompressedSize64 != e.Size || f.Method != e.Method {
			return fail("%q: central directory does not match what was written", e.Path)
		}
		n := e.DataOffset - e.HeaderOffset
		if e.HeaderOffset < 0 || n < 30 || n > 30+65535*2 || e.DataOffset > size {
			return fail("%q: invalid header offsets", e.Path)
		}
		hdr := make([]byte, n)
		if _, err := r.ReadAt(hdr, e.HeaderOffset); err != nil {
			return fail("%q: read local header: %v", e.Path, err)
		}
		nameLen := int(le16(hdr[26:]))
		extraLen := int(le16(hdr[28:]))
		flags := le16(hdr[6:])
		if le32(hdr) != 0x04034b50 || 30+nameLen+extraLen != int(n) || le16(hdr[4:]) != f.ReaderVersion ||
			string(hdr[30:30+nameLen]) != e.Path || le16(hdr[8:]) != e.Method || flags != f.Flags ||
			le16(hdr[10:]) != f.ModifiedTime || le16(hdr[12:]) != f.ModifiedDate ||
			!bytes.HasPrefix(f.Extra, hdr[30+nameLen:]) {
			return fail("%q: local file header is damaged", e.Path)
		}
		if off, err := f.DataOffset(); err != nil || off != e.DataOffset {
			return fail("%q: data offset does not match", e.Path)
		}
		if flags&0x8 == 0 {
			if le32(hdr[14:]) != e.CRC32 || uint64(le32(hdr[18:])) != e.CompressedSize || uint64(le32(hdr[22:])) != e.Size {
				return fail("%q: local file header is damaged", e.Path)
			}
			continue
		}
		if le32(hdr[14:]) != 0 || le32(hdr[18:]) != 0 || le32(hdr[22:]) != 0 {
			return fail("%q: local file header is damaged", e.Path)
		}
		// Data descriptor: signature, CRC, then 32- or 64-bit sizes.
		zip64 := e.CompressedSize >= 0xffffffff || e.Size >= 0xffffffff
		dd := make([]byte, 24)
		ddLen := 16
		if zip64 {
			ddLen = 24
		}
		end := e.DataOffset + int64(min(e.CompressedSize, 1<<62))
		if end+int64(ddLen) > size {
			return fail("%q: data descriptor missing", e.Path)
		}
		if _, err := r.ReadAt(dd[:ddLen], end); err != nil {
			return fail("%q: read data descriptor: %v", e.Path, err)
		}
		ok := le32(dd) == 0x08074b50 && le32(dd[4:]) == e.CRC32
		if zip64 {
			ok = ok && le64(dd[8:]) == e.CompressedSize && le64(dd[16:]) == e.Size
		} else {
			ok = ok && uint64(le32(dd[8:])) == e.CompressedSize && uint64(le32(dd[12:])) == e.Size
		}
		if !ok {
			return fail("%q: data descriptor is damaged", e.Path)
		}
	}
	return nil
}
