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

//go:build integration

package cli

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"filippo.io/age"

	"github.com/macsnoeren/backup-usb/internal/app"
	"github.com/macsnoeren/backup-usb/internal/crypt"
	"github.com/macsnoeren/backup-usb/internal/vault"
)

func (w *world) vaultKeys() []vault.Key {
	w.stdin = testPassword + "\n"
	out, _ := w.mustRun(0, "keys", "status", "--json", "--password-stdin", "--vault", w.vaultPath())
	var keys []vault.Key
	if err := json.Unmarshal([]byte(out), &keys); err != nil {
		w.t.Fatal(err)
	}
	return keys
}

func (w *world) identities(id string) age.Identity {
	v, err := vault.Open(w.vaultPath(), secmemFrom(testPassword))
	if err != nil {
		w.t.Fatal(err)
	}
	defer v.Close()
	i, err := v.Identity(id)
	if err != nil {
		w.t.Fatal(err)
	}
	return i
}

func TestKeyRotationOverTwoDisks(t *testing.T) {
	w := newWorld(t)
	w.keysInit()
	w.enroll("BACKUP1")
	w.enroll("BACKUP2")
	w.mustRun(0, "run")                 // BACKUP1, week 47, key k1
	w.clock.Advance(7 * 24 * time.Hour) // week 48
	w.mustRun(0, "run")                 // BACKUP2, key k1
	k1 := w.identities("k1")

	w.stdin = testPassword + "\n"
	out, _ := w.mustRun(0, "keys", "rotate", "--password-stdin", "--vault", w.vaultPath())
	if !strings.Contains(out, "New operational key k2") || !strings.Contains(out, "BACKUP1, BACKUP2") {
		t.Fatal(out)
	}
	keys := w.vaultKeys()
	if len(keys) != 2 || keys[0].Status != "retired" || len(keys[0].PendingDisks) != 2 || keys[1].Status != "active" {
		t.Fatalf("vault %+v", keys)
	}
	k2 := w.identities("k2")

	// A new backup uses the new key at once, and the run reports that the
	// disk still has old backups (without asking for a password).
	w.clock.Advance(7 * 24 * time.Hour) // week 49: BACKUP1
	out, _ = w.mustRun(0, "run", "--json")
	var rep app.RunReport
	json.Unmarshal([]byte(out), &rep)
	if !strings.Contains(strings.Join(rep.Messages, " "), "must be updated by the operator") || !strings.Contains(strings.Join(rep.MessageKeys, " "), "msg.needsUpdate") {
		t.Fatalf("no update notice: %+v", rep.Messages)
	}
	root, _ := os.OpenRoot(w.diskFile("BACKUP1", "Backup"))
	if !crypt.OpensWith(root, "2026/49/Data.zip.age", k2) || crypt.OpensWith(root, "2026/49/Data.zip.age", k1) {
		t.Fatal("new backup not for the new key only")
	}
	if crypt.OpensWith(root, "2026/47/Data.zip.age", k2) {
		t.Fatal("old backup already re-wrapped")
	}

	// Re-wrap BACKUP1 (BACKUP2 unplugged).
	w.unplug("BACKUP2")
	w.stdin = testPassword + "\n"
	out, _ = w.mustRun(0, "keys", "rewrap", "--password-stdin", "--vault", w.vaultPath())
	if !strings.Contains(out, "BACKUP1: 4 files re-wrapped, 4 already current") {
		t.Fatal(out)
	}
	for _, f := range []string{"2026/47/Data.zip.age", "2026/47/Data.index.age", "2026/47/Books.zip.age"} {
		if crypt.OpensWith(root, f, k1) || !crypt.OpensWith(root, f, k2) {
			t.Fatalf("%s not re-wrapped", f)
		}
	}
	root.Close()
	w.mustRun(0, "verify", "--all")                        // sha256 files updated
	w.mustRun(0, "verify", "--all", "--identity", w.bgKey) // break-glass still works
	if keys := w.vaultKeys(); len(keys[0].PendingDisks) != 1 || keys[0].PendingDisks[0] != "BACKUP2" {
		t.Fatalf("pending %+v", keys[0].PendingDisks)
	}
	out, _ = w.mustRun(0, "status")
	if strings.Contains(out, "must be updated") {
		t.Fatal("BACKUP1 still marked for update")
	}

	// Then BACKUP2: afterwards the old key is free; answer yes.
	w.addBack("BACKUP2")
	w.disks.Remove("BACKUP1")
	w.stdin = testPassword + "\ny\n"
	out, _ = w.mustRun(0, "keys", "rewrap", "--password-stdin", "--vault", w.vaultPath())
	if !strings.Contains(out, "Key k1 deleted") {
		t.Fatal(out)
	}
	if keys := w.vaultKeys(); len(keys) != 1 || keys[0].ID != "k2" {
		t.Fatalf("vault %+v", keys)
	}
	w.stdin = testPassword + "\n"
	w.mustRun(0, "verify", "--all", "--password-stdin", "--vault", w.vaultPath())
}

func TestRewrapWrongPassword(t *testing.T) {
	w := newWorld(t)
	w.keysInit()
	w.enroll("BACKUP1")
	w.mustRun(0, "run")
	w.stdin = "wrong password!!\n"
	w.mustRun(13, "keys", "rewrap", "--password-stdin", "--vault", w.vaultPath())
}

func TestRotateFull(t *testing.T) {
	w := newWorld(t)
	w.keysInit()
	w.enroll("BACKUP1")
	w.mustRun(0, "run")
	k1 := w.identities("k1")
	root, _ := os.OpenRoot(w.diskFile("BACKUP1", "Backup"))
	defer root.Close()
	before, err := crypt.FileKeyOf(root, "2026/47/Data.zip.age", k1)
	if err != nil {
		t.Fatal(err)
	}
	w.stdin = testPassword + "\n"
	w.mustRun(0, "keys", "rotate", "--full", "--password-stdin", "--vault", w.vaultPath())
	w.stdin = testPassword + "\n"
	w.mustRun(0, "keys", "rewrap", "--keep-old", "--password-stdin", "--vault", w.vaultPath())
	after, err := crypt.FileKeyOf(root, "2026/47/Data.zip.age", w.identities("k2"))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) == string(after) {
		t.Fatal("file key unchanged after rotate --full")
	}
	if crypt.OpensWith(root, "2026/47/Data.zip.age", k1) {
		t.Fatal("old key still opens it")
	}
	w.mustRun(0, "verify", "--all", "--identity", w.bgKey)
}
