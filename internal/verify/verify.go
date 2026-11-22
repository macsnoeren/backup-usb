package verify

import (
	"archive/zip"
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
