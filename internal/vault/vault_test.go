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

package vault

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"filippo.io/age"

	"github.com/macsnoeren/backup-usb/internal/secmem"
	"github.com/macsnoeren/backup-usb/internal/testutil"
)

func init() { ScryptWorkFactor = 10 }

func pw(s string) *secmem.Buffer { return secmem.From([]byte(s)) }

func newVault(t *testing.T) *Vault {
	v, err := New(testutil.TestSigningSeed(t, "signing"), testutil.TestRecipient(t, "breakglass"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v.AddKey("k1", []byte(testutil.TestIdentity(t, "operational")), time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "recovery.vault")
	v := newVault(t)
	if err := v.Save(path, pw("correct horse battery")); err != nil {
		t.Fatal(err)
	}
	st, _ := os.Stat(path)
	if runtime.GOOS != "windows" && st.Mode().Perm() != 0o600 { // Windows has ACLs
		t.Errorf("vault mode %v", st.Mode().Perm())
	}
	raw, _ := os.ReadFile(path)
	if bytes.Contains(raw, []byte("AGE-SECRET-KEY")) || !bytes.HasPrefix(raw, []byte("age-encryption.org/v1")) {
		t.Fatal("vault is not encrypted")
	}
	back, err := Open(path, pw("correct horse battery"))
	if err != nil {
		t.Fatal(err)
	}
	defer back.Close()
	k := back.Active()
	if k == nil || k.ID != "k1" || k.Fingerprint != Fingerprint(testutil.TestRecipient(t, "operational")) || len(k.Fingerprint) != 16 {
		t.Fatalf("key %+v", k)
	}
	if back.Meta.BreakglassRecipient != testutil.TestRecipient(t, "breakglass") {
		t.Fatal("break-glass recipient")
	}
	ids, err := back.Identities()
	if err != nil || len(ids) != 1 {
		t.Fatal(err)
	}
	sk := back.SigningKey()
	if sk.Len() != 64 {
		t.Fatal("signing key")
	}
	sk.Wipe()
	pub, err := back.SigningPublic()
	if err != nil || len(pub) != 32 {
		t.Fatal(err)
	}
}

func TestWrongPassword(t *testing.T) {
	path := filepath.Join(t.TempDir(), "recovery.vault")
	if err := newVault(t).Save(path, pw("correct horse battery")); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(path, pw("wrong password!!")); !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("err = %v", err)
	}
	if _, err := Open(filepath.Join(t.TempDir(), "none"), pw("x")); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	raw[len(raw)-5] ^= 1
	os.WriteFile(path, raw, 0o600)
	if _, err := Open(path, pw("correct horse battery")); !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("damaged vault: %v", err)
	}
}

func TestBuffersWipedAfterClose(t *testing.T) {
	v := newVault(t)
	id := v.identities[0].Bytes()
	seed := v.signing.Bytes()
	v.Close()
	for _, b := range [][]byte{id[:cap(id)], seed[:cap(seed)]} {
		for _, c := range b {
			if c != 0 {
				t.Fatal("secret left in memory after Close")
			}
		}
	}
}

func TestChangePasswordAndHistory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "recovery.vault")
	v := newVault(t)
	if _, err := v.AddKey("k2", []byte(testutil.TestIdentity(t, "operational2")), time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if err := v.Save(path, pw("first password!")); err != nil {
		t.Fatal(err)
	}
	if err := ChangePassword(path, pw("first password!"), pw("second password!")); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(path, pw("first password!")); !errors.Is(err, ErrWrongPassword) {
		t.Fatal("old password still works")
	}
	back, err := Open(path, pw("second password!"))
	if err != nil {
		t.Fatal(err)
	}
	defer back.Close()
	if len(back.Meta.Keys) != 2 || back.Key("k1").Status != StatusRetired || back.Key("k1").Retired != "2026-12-01" || back.Active().ID != "k2" {
		t.Fatalf("history %+v", back.Meta.Keys)
	}
	if err := back.RemoveKey("k2"); err == nil {
		t.Fatal("active key removed")
	}
	if err := back.RemoveKey("k1"); err != nil || len(back.identities) != 1 {
		t.Fatal(err)
	}
}

// The vault is a valid age identity file: the age library (and therefore
// "age -d -i recovery.vault") can use it after decrypting with the
// password.
func TestVaultIsAgeIdentityFile(t *testing.T) {
	v := newVault(t)
	pt, err := v.plaintext()
	if err != nil {
		t.Fatal(err)
	}
	ids, err := age.ParseIdentities(bytes.NewReader(pt.Bytes()))
	if err != nil || len(ids) != 1 {
		t.Fatalf("age cannot parse the vault: %v", err)
	}
	rcp, _ := age.ParseHybridRecipient(testutil.TestRecipient(t, "operational"))
	var buf bytes.Buffer
	w, _ := age.Encrypt(&buf, rcp)
	w.Write([]byte("hello"))
	w.Close()
	if _, err := age.Decrypt(&buf, ids...); err != nil {
		t.Fatal(err)
	}
}

func FuzzParse(f *testing.F) {
	v, _ := New(make([]byte, 32), "age1pq1x")
	pt, _ := v.plaintext()
	f.Add(append([]byte(nil), pt.Bytes()...))
	f.Fuzz(func(t *testing.T, data []byte) {
		if v, err := parse(data); err == nil {
			v.Close()
		}
	})
}
