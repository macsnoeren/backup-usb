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
	"errors"
	"io/fs"
	"path"

	"filippo.io/age"

	"github.com/macsnoeren/backup-usb/internal/archive"
	"github.com/macsnoeren/backup-usb/internal/crypt"
	"github.com/macsnoeren/backup-usb/internal/fsx"
)

// EncJob is one encrypted archive to write.
type EncJob struct {
	ZipJob
	Recipients []age.Recipient
}

// WriteVerifiedEncrypted writes job.Rel (ending in .zip.age) as a part file,
// with the zip checked in the stream, then reads the encrypted file back
// from the disk and compares its SHA-256 with the one computed while
// writing. No private key is needed. Only then the sha256 file is written
// and the part file renamed.
func WriteVerifiedEncrypted(ctx context.Context, job EncJob) (out *ZipOutcome, err error) {
	part := job.Rel + fsx.PartSuffix
	shaRel := job.Rel + ".sha256"
	defer func() {
		if err != nil {
			_ = job.Root.Remove(part)
			_ = job.Root.Remove(shaRel + fsx.PartSuffix)
		}
	}()
	res, err := crypt.WriteEncrypted(ctx, job.Src, job.Root, job.Rel, job.Recipients, job.Opts)
	if err != nil {
		if errors.Is(err, crypt.ErrStreamCheck) {
			return nil, Errorf(KindVerify, err, "err.verify", path.Base(job.Rel))
		}
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
	sum, size, err := HashFile(ctx, job.Root, part)
	if err != nil {
		return nil, Errorf(KindIO, err, "err.read")
	}
	if sum != res.SHA256 || size != res.Size {
		return nil, Errorf(KindVerify, errors.New("encrypted file read back from disk differs from what was written"), "err.verify", name)
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
		return nil, Errorf(KindIO, err, "err.write")
	}
	return &ZipOutcome{Result: res.Archive, SHA256: sum, Size: size}, nil
}
