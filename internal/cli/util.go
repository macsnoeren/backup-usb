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

package cli

import (
	"errors"
	"io"
	"os"

	"github.com/macsnoeren/backup-usb/internal/app"
	"github.com/macsnoeren/backup-usb/internal/msg"
	"github.com/macsnoeren/backup-usb/internal/secmem"
	"github.com/macsnoeren/backup-usb/internal/vault"
)

func removeFile(p string) error { return os.Remove(p) }

func fingerprintOf(recipient string) string { return vault.Fingerprint(recipient) }

func asAppErr(err error, target **app.Error) bool { return errors.As(err, target) }

// readSecretFile reads a key file straight into a locked buffer, without
// intermediate copies.
func readSecretFile(p string, maxSize int64) (*secmem.Buffer, error) {
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }() // read-only
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() || st.Size() > maxSize {
		return nil, errors.New("not a key file")
	}
	buf := secmem.New(int(st.Size()))
	if _, err := io.ReadFull(f, buf.Bytes()); err != nil {
		buf.Wipe()
		return nil, err
	}
	return buf, nil
}

func msgLang(s string) msg.Lang { return msg.Lang(s) }
