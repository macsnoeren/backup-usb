package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/macsnoeren/backup-usb/internal/archive"
	"github.com/macsnoeren/backup-usb/internal/fsx"
	"github.com/macsnoeren/backup-usb/internal/verify"
)

// ZipJob is one unencrypted zip to write.
type ZipJob struct {
	Src    string
	Dest   string // final path, e.g. <disk>/Backup/2026/47/Data.zip
	Opts   archive.Options
	Verify bool
}

// ZipOutcome is the result of a written, verified and renamed zip.
type ZipOutcome struct {
	Result *archive.Result
	SHA256 string
	Size   int64
}

// testHookAfterWrite, if set, runs after the part file is written. Tests
// use it to simulate failures.
var testHookAfterWrite func(part string) error

// WriteVerifiedZip writes job.Dest+".part", verifies it, writes the
// sha256 file and only then renames the part file. On any failure nothing
// new is left behind: no .part file and no half final file.
func WriteVerifiedZip(ctx context.Context, job ZipJob) (out *ZipOutcome, err error) {
	part := job.Dest + fsx.PartSuffix
	shaPath := job.Dest + ".sha256"
	defer func() {
		if err != nil {
			_ = os.Remove(part)
			_ = os.Remove(shaPath + fsx.PartSuffix)
		}
	}()
	res, err := archive.WriteFile(ctx, job.Src, job.Dest, job.Opts)
	if err != nil {
		return nil, classifyWriteErr(err)
	}
	if testHookAfterWrite != nil {
		if err := testHookAfterWrite(part); err != nil {
			return nil, Errorf(KindIO, err, "err.write")
		}
	}
	if job.Verify {
		if _, err := verify.File(ctx, part, verify.Options{ExpectedFiles: res.Files, Entries: res.AllEntries()}); err != nil {
			return nil, Errorf(KindVerify, err, "err.verify", filepath.Base(job.Dest))
		}
	}
	// Read the whole zip back from the disk: it must be exactly what was
	// written.
	sum, size, err := HashFile(ctx, part)
	if err != nil {
		return nil, Errorf(KindIO, err, "err.read")
	}
	if sum != res.StreamSHA256 || size != res.ZipSize {
		return nil, Errorf(KindVerify, errors.New("zip read back from disk differs from what was written"), "err.verify", filepath.Base(job.Dest))
	}
	line := archive.ManifestLine(sum, filepath.Base(job.Dest))
	if err := writeSynced(shaPath+fsx.PartSuffix, []byte(line)); err != nil {
		return nil, Errorf(KindIO, err, "err.write")
	}
	if err := fsx.Rename(part, job.Dest); err != nil {
		return nil, Errorf(KindIO, err, "err.write")
	}
	if err := fsx.Rename(shaPath+fsx.PartSuffix, shaPath); err != nil {
		return nil, Errorf(KindIO, err, "err.write")
	}
	return &ZipOutcome{Result: res, SHA256: sum, Size: size}, nil
}

func classifyWriteErr(err error) error {
	switch {
	case errors.Is(err, context.Canceled):
		return Errorf(KindIO, err, "err.cancelled")
	case errors.Is(err, archive.ErrLocked):
		return Errorf(KindIO, err, "err.locked")
	case errors.Is(err, archive.ErrReadFailed):
		return Errorf(KindIO, err, "err.read")
	}
	return Errorf(KindIO, err, "err.write")
}

// writeSynced creates path with data and syncs it.
func writeSynced(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("create %s: %w", filepath.Base(path), err)
	}
	_, werr := f.Write(data)
	serr := f.Sync()
	cerr := f.Close()
	if err := errors.Join(werr, serr, cerr); err != nil {
		return fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}
	return nil
}

// HashFile returns the SHA-256 and size of a file, read from disk.
func HashFile(ctx context.Context, path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, fmt.Errorf("open %s: %w", filepath.Base(path), err)
	}
	defer func() { _ = f.Close() }() // read-only
	h := sha256.New()
	n, err := io.Copy(h, ctxReader{ctx, f})
	if err != nil {
		return "", 0, fmt.Errorf("read %s: %w", filepath.Base(path), err)
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

type ctxReader struct {
	ctx context.Context
	r   io.Reader
}

func (c ctxReader) Read(p []byte) (int, error) {
	if err := c.ctx.Err(); err != nil {
		return 0, err
	}
	return c.r.Read(p)
}

// partSuffixes are the leftover files of an interrupted run that
// CleanupParts may remove. Nothing else is ever touched.
var partSuffixes = []string{".zip.part", ".zip.age.part", ".sha256.part", ".index.json.part", ".index.age.part", ".txt.part", ".json.part"}

// CleanupParts removes leftover part files of interrupted runs under dir
// (the backup folder on a disk). All access goes through an os.Root, so a
// symlink on the disk can never lead outside dir. It returns the removed
// files relative to dir.
func CleanupParts(dir string) ([]string, error) {
	root, err := os.OpenRoot(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open backup folder: %w", err)
	}
	defer func() { _ = root.Close() }()
	var removed []string
	err = fs.WalkDir(root.FS(), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.Type().IsRegular() {
			return nil
		}
		for _, s := range partSuffixes {
			if strings.HasSuffix(d.Name(), s) {
				if err := root.Remove(p); err != nil {
					return fmt.Errorf("remove %s: %w", d.Name(), err)
				}
				removed = append(removed, p)
				break
			}
		}
		return nil
	})
	return removed, err
}
