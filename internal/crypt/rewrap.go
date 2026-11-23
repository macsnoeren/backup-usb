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
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"filippo.io/age"

	"github.com/macsnoeren/backup-usb/internal/fsx"
	"github.com/macsnoeren/backup-usb/internal/secmem"
)

// ErrDamaged means the file to re-wrap is not the file that was written.
var ErrDamaged = errors.New("file is damaged")

// ErrDecrypt means the file key could not be unwrapped with the given keys
// (wrong key or damaged header).
var ErrDecrypt = errors.New("cannot unwrap the file key")

// Rewrap re-wraps the age file rel inside root for new recipients: the
// file key is unwrapped with oldIDs, wrapped for recipients, the header MAC
// recomputed and the payload copied unchanged to rel+".part". The part file
// is then checked with check (an identity for one of the new recipients):
// it must yield the same file key and the payload must be byte-equal. When
// expectedSHA is set, the original file must have that SHA-256 (from its
// .sha256 file): a damaged file is never re-wrapped, so its damage cannot
// be hidden behind a new hash. Only then the part file replaces the
// original. On any failure the original is left as it was.
func Rewrap(root *os.Root, rel string, oldIDs []age.Identity, recipients []age.Recipient, check age.Identity, expectedSHA string) (err error) {
	f, err := fsx.OpenRegular(root, rel)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }() // read-only
	hdr, payload, err := ReadHeader(f)
	if err != nil {
		return err
	}
	fk, err := age.DecryptHeader(hdr.Marshal(), oldIDs...)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrDecrypt, err)
	}
	fileKey := secmem.From(fk)
	defer fileKey.Wipe()
	nh := &Header{}
	for _, r := range recipients {
		ss, err := r.Wrap(fileKey.Bytes())
		if err != nil {
			return fmt.Errorf("wrap file key: %w", err)
		}
		nh.Stanzas = append(nh.Stanzas, ss...)
	}
	if err := nh.SetMAC(fileKey.Bytes()); err != nil {
		return err
	}

	part := rel + fsx.PartSuffix
	out, err := fsx.CreateExclusive(root, part, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = root.Remove(part)
		}
	}()
	h := sha256.New()
	orig := sha256.New()
	orig.Write(hdr.Marshal()) // byte-equal to the header on disk (strict parser)
	_, werr := out.Write(nh.Marshal())
	if werr == nil {
		_, werr = io.Copy(io.MultiWriter(out, h, orig), payload)
	}
	serr := out.Sync()
	cerr := out.Close()
	if err := errors.Join(werr, serr, cerr); err != nil {
		return fmt.Errorf("write %s: %w", part, err)
	}
	want := h.Sum(nil)
	if expectedSHA != "" && hex.EncodeToString(orig.Sum(nil)) != strings.ToLower(expectedSHA) {
		return fmt.Errorf("%w: the file does not match its .sha256", ErrDamaged)
	}

	// Check the new file with the new key before it replaces the old one.
	if err := checkRewrapped(root, part, fileKey.Bytes(), check, want); err != nil {
		return err
	}
	return fsx.RootRename(root, part, rel)
}

func checkRewrapped(root *os.Root, part string, fileKey []byte, check age.Identity, payloadSum []byte) error {
	f, err := fsx.OpenRegular(root, part)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }() // read-only
	hdr, payload, err := ReadHeader(f)
	if err != nil {
		return err
	}
	fk, err := age.DecryptHeader(hdr.Marshal(), check)
	if err != nil {
		return fmt.Errorf("re-wrapped header does not open with the new key: %w", err)
	}
	same := subtle.ConstantTimeCompare(fk, fileKey) == 1
	secmem.Wipe(fk)
	if !same {
		return errors.New("re-wrapped header holds another file key")
	}
	h := sha256.New()
	if _, err := io.Copy(h, payload); err != nil {
		return fmt.Errorf("read back: %w", err)
	}
	if subtle.ConstantTimeCompare(h.Sum(nil), payloadSum) != 1 {
		return errors.New("payload changed while re-wrapping")
	}
	return nil
}
