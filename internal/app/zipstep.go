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
	"path"
	"strings"

	"github.com/macsnoeren/backup-usb/internal/archive"
	"github.com/macsnoeren/backup-usb/internal/fsx"
	"github.com/macsnoeren/backup-usb/internal/verify"
)

// ZipJob is one unencrypted zip to write.
type ZipJob struct {
	Src string
	// Root is the backup folder on the disk; Rel the zip inside it, for
	// example "2026/47/Data.zip".
	Root   *os.Root
	Rel    string
	Opts   archive.Options
	Verify bool
	// OnVerify is called when writing is done and checking starts.
	OnVerify func()
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

// WriteVerifiedZip writes job.Rel+".part", verifies it, writes the sha256
// file and only then renames the part file. On a failure before the rename
// nothing new is left behind: no .part file and no half final file. The old
// .sha256 is removed before the rename, so a new zip is never next to a hash
// that does not belong to it.
func WriteVerifiedZip(ctx context.Context, job ZipJob) (out *ZipOutcome, err error) {
	part := job.Rel + fsx.PartSuffix
	shaRel := job.Rel + ".sha256"
	defer func() {
		if err != nil {
			_ = job.Root.Remove(part)
			_ = job.Root.Remove(shaRel + fsx.PartSuffix)
		}
	}()
	res, err := archive.WriteFile(ctx, job.Src, job.Root, job.Rel, job.Opts)
	if err != nil {
		return nil, classifyWriteErr(err)
	}
	if testHookAfterWrite != nil {
		if err := testHookAfterWrite(part); err != nil {
			return nil, Errorf(KindIO, err, "err.write")
		}
	}
	name := path.Base(job.Rel)
	if job.OnVerify != nil {
		job.OnVerify()
	}
	if job.Verify {
		if err := verifyInRoot(ctx, job.Root, part, verify.Options{ExpectedFiles: res.Files, Entries: res.AllEntries()}); err != nil {
			return nil, Errorf(KindVerify, err, "err.verify", name)
		}
	}
	// Read the whole zip back from the disk: it must be exactly what was
	// written.
	sum, size, err := HashFile(ctx, job.Root, part)
	if err != nil {
		return nil, Errorf(KindIO, err, "err.read")
	}
	if sum != res.StreamSHA256 || size != res.ZipSize {
		return nil, Errorf(KindVerify, errors.New("zip read back from disk differs from what was written"), "err.verify", name)
	}
	if err := writeSynced(job.Root, shaRel+fsx.PartSuffix, []byte(archive.ManifestLine(sum, name))); err != nil {
		return nil, Errorf(KindIO, err, "err.write")
	}
	if err := job.Root.Remove(shaRel); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, Errorf(KindIO, err, "err.write")
	}
	if err := fsx.RootRename(job.Root, part, job.Rel); err != nil {
		return nil, Errorf(KindIO, err, "err.write")
	}
	if err := fsx.RootRename(job.Root, shaRel+fsx.PartSuffix, shaRel); err != nil {
		// The zip is in place but has no hash file: report the failure;
		// the next run replaces it.
		return nil, Errorf(KindIO, err, "err.write")
	}
	return &ZipOutcome{Result: res, SHA256: sum, Size: size}, nil
}

func verifyInRoot(ctx context.Context, root *os.Root, rel string, opts verify.Options) error {
	f, err := fsx.OpenRegular(root, rel)
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}
	defer func() { _ = f.Close() }() // read-only
	st, err := f.Stat()
	if err != nil {
		return fmt.Errorf("stat zip: %w", err)
	}
	_, err = verify.Zip(ctx, f, st.Size(), opts)
	return err
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

// writeSynced creates rel inside root exclusively with data and syncs it.
func writeSynced(root *os.Root, rel string, data []byte) error {
	f, err := fsx.CreateExclusive(root, rel, 0o600)
	if err != nil {
		return err
	}
	_, werr := f.Write(data)
	serr := f.Sync()
	cerr := f.Close()
	if err := errors.Join(werr, serr, cerr); err != nil {
		return fmt.Errorf("write %s: %w", path.Base(rel), err)
	}
	return nil
}

// HashFile returns the SHA-256 and size of a file inside root, read from
// disk.
func HashFile(ctx context.Context, root *os.Root, rel string) (string, int64, error) {
	f, err := fsx.OpenRegular(root, rel)
	if err != nil {
		return "", 0, fmt.Errorf("open %s: %w", path.Base(rel), err)
	}
	defer func() { _ = f.Close() }() // read-only
	h := sha256.New()
	n, err := io.Copy(h, ctxReader{ctx, f})
	if err != nil {
		return "", 0, fmt.Errorf("read %s: %w", path.Base(rel), err)
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
var partSuffixes = []string{".zip.part", ".zip.age.part", ".sha256.part", ".index.gz.part", ".index.age.part", ".txt.part", ".json.part"}

// CleanupParts removes leftover part files of interrupted runs inside root
// (the backup folder on a disk). All access goes through the os.Root, so a
// symlink on the disk can never lead outside it. It returns the removed
// files.
func CleanupParts(root *os.Root) ([]string, error) {
	var removed []string
	err := fs.WalkDir(root.FS(), ".", func(p string, d fs.DirEntry, err error) error {
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
