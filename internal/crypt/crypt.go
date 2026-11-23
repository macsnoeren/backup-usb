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

package crypt

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"

	"filippo.io/age"

	"github.com/macsnoeren/backup-usb/internal/archive"
	"github.com/macsnoeren/backup-usb/internal/fsx"
	"github.com/macsnoeren/backup-usb/internal/verify"
)

// Suffix is appended to encrypted files: Data.zip.age.
const Suffix = ".age"

// Result describes an encrypted archive that was written.
type Result struct {
	Archive *archive.Result
	// SHA256 and Size describe the encrypted bytes as written.
	SHA256 string
	Size   int64
}

type countHash struct {
	w io.Writer
	h interface {
		io.Writer
		Sum([]byte) []byte
	}
	n int64
}

func (c *countHash) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	_, _ = c.h.Write(p[:n]) // a hash never fails
	c.n += int64(n)
	return n, err
}

type tee struct{ a, b io.Writer }

func (t tee) Write(p []byte) (int, error) {
	if n, err := t.a.Write(p); err != nil {
		return n, err
	}
	if _, err := t.b.Write(p); err != nil {
		return 0, err
	}
	return len(p), nil
}

// ErrStreamCheck wraps a failed in-stream zip check.
var ErrStreamCheck = errors.New("zip check in the stream failed")

// WriteEncrypted archives src as one stream: files, zip, age, disk. It
// writes rel+".part" inside root (rel ends in .zip.age). The zip never
// exists unencrypted on the disk: it is checked in memory while it is
// produced (CRC-32 and SHA-256 per file, against the manifest), and the
// SHA-256 of the encrypted bytes is computed while writing. On error the
// part file is removed.
func WriteEncrypted(ctx context.Context, src string, root *os.Root, rel string, recipients []age.Recipient, opts archive.Options) (res *Result, err error) {
	part := rel + fsx.PartSuffix
	f, err := fsx.CreateExclusive(root, part, 0o600)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := f.Close(); err == nil && cerr != nil {
			err = fmt.Errorf("close %s: %w", part, cerr)
		}
		if err != nil {
			_ = root.Remove(part)
			res = nil
		}
	}()
	out := &countHash{w: f, h: sha256.New()}
	aw, err := age.Encrypt(out, recipients...)
	if err != nil {
		return nil, fmt.Errorf("encrypt: %w", err)
	}
	vs := verify.NewStream()
	opts.StoredEnd = vs.StoredEnd
	ares, err := archive.Write(ctx, src, tee{aw, vs.Writer()}, opts)
	if err != nil {
		vs.Abort(err)
		return nil, err
	}
	entries, verr := vs.Finish()
	if verr != nil {
		return nil, fmt.Errorf("%w: %w", ErrStreamCheck, verr)
	}
	if err := compareEntries(ares, entries); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrStreamCheck, err)
	}
	if err := aw.Close(); err != nil {
		return nil, fmt.Errorf("finish encryption: %w", err)
	}
	if err := f.Sync(); err != nil {
		return nil, fmt.Errorf("sync %s: %w", part, err)
	}
	return &Result{Archive: ares, SHA256: hex.EncodeToString(out.h.Sum(nil)), Size: out.n}, nil
}

// compareEntries checks that the verifier saw exactly what the writer
// wrote.
func compareEntries(res *archive.Result, got []verify.StreamEntry) error {
	want := res.AllEntries()
	if len(want) != len(got) {
		return fmt.Errorf("zip has %d entries, the writer wrote %d", len(got), len(want))
	}
	for i, w := range want {
		g := got[i]
		if g.Name != w.Path || g.CRC32 != w.CRC32 || g.Size != w.Size || g.CompressedSize != w.CompressedSize ||
			g.DataOffset != w.DataOffset || g.HeaderOffset != w.HeaderOffset || (!w.Dir && g.SHA256 != w.SHA256) {
			return fmt.Errorf("entry %q differs from what was written", w.Path)
		}
	}
	return nil
}
