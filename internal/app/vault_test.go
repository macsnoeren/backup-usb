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
	"path/filepath"
	"testing"
	"time"

	"github.com/macsnoeren/backup-usb/internal/secmem"
	"github.com/macsnoeren/backup-usb/internal/testutil"
	"github.com/macsnoeren/backup-usb/internal/vault"
)

func TestOpenVaultKinds(t *testing.T) {
	vault.ScryptWorkFactor = 10
	path := filepath.Join(t.TempDir(), "recovery.vault")
	v, _ := vault.New(testutil.TestSigningSeed(t, "signing"), testutil.TestRecipient(t, "breakglass"))
	v.AddKey("k1", []byte(testutil.TestIdentity(t, "operational")), time.Now())
	if err := v.Save(path, secmem.From([]byte("a good password"))); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenVault(path, secmem.From([]byte("a bad password!"))); KindOf(err) != KindDecrypt {
		t.Fatalf("wrong password: %v", err)
	}
	if _, err := OpenVault(path+"x", secmem.From([]byte("a good password"))); KindOf(err) != KindDecrypt {
		t.Fatalf("missing: %v", err)
	}
	got, err := OpenVault(path, secmem.From([]byte("a good password")))
	if err != nil {
		t.Fatal(err)
	}
	got.Close()
}
