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
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"filippo.io/age"

	"github.com/macsnoeren/backup-usb/internal/app"
	"github.com/macsnoeren/backup-usb/internal/testutil"
)

func TestRunEncrypted(t *testing.T) {
	w := newWorld(t)
	w.enrollUnsigned("BACKUP1", "BACKUP2")
	bg := w.keysInit()
	out, _ := w.mustRun(0, "run", "--json")
	var rep app.RunReport
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatal(err)
	}
	rec, _ := app.ReadRecipients(w.cfgDir + "/usb-backup.recipients")
	if !rep.Encrypted || len(rep.Keys) != 2 || rep.Keys[0] != rec.Fingerprints()[0] {
		t.Fatalf("report %+v", rep)
	}
	week := "Backup/2026/47/"
	if exists(w.diskFile("BACKUP1", week+"Data.zip")) || !exists(w.diskFile("BACKUP1", week+"Data.zip.age")) || !exists(w.diskFile("BACKUP1", week+"Data.zip.age.sha256")) {
		t.Fatal("expected only encrypted archives")
	}
	// Nothing readable on the disk: no file names, no zip.
	tree := testutil.Tree(t, w.disks.Disk("BACKUP1").Mount)
	for name, content := range tree {
		for _, leak := range []string{"todo.txt", "buy milk", "PK\x03\x04", "q1.xlsx"} {
			if strings.Contains(name, leak) || strings.Contains(content, leak) {
				t.Fatalf("%q readable in %s", leak, name)
			}
		}
	}
	report := tree["Backup/2026/47/report.txt"]
	keysJSON := tree["Backup/keys.json"]
	for _, fp := range rec.Fingerprints() {
		if !strings.Contains(report, fp) || !strings.Contains(keysJSON, fp) {
			t.Fatalf("fingerprint %s missing in report.txt or keys.json", fp)
		}
	}
	// The break-glass key decrypts it.
	bgData, _ := os.ReadFile(bg)
	ids, _ := age.ParseIdentities(bytes.NewReader(bgData))
	f, _ := os.Open(w.diskFile("BACKUP1", week+"Books.zip.age"))
	defer f.Close()
	r, err := age.Decrypt(f, ids...)
	if err != nil {
		t.Fatal(err)
	}
	if data, err := io.ReadAll(r); err != nil || !bytes.HasPrefix(data, []byte("PK\x03\x04")) {
		t.Fatal("decrypted content is not a zip", err)
	}
}

func TestRunEncryptionRequiredButNoKeys(t *testing.T) {
	w := newWorld(t)
	w.enrollUnsigned("BACKUP1")
	w.set("encryption", "age")
	_, errOut := w.mustRun(10, "run")
	if !strings.Contains(errOut, "encryption is required") {
		t.Fatalf("stderr %q", errOut)
	}
	if exists(w.diskFile("BACKUP1", "Backup/2026")) {
		t.Fatal("something was written")
	}
	// encryption: none ignores the recipients file.
	w.keysInit()
	w.set("encryption", "none")
	w.mustRun(0, "run")
	if !exists(w.diskFile("BACKUP1", "Backup/2026/47/Data.zip")) {
		t.Fatal("encryption none")
	}
}

func TestRecipientsAreSigned(t *testing.T) {
	w := newWorld(t)
	w.enrollUnsigned("BACKUP1")
	w.keysInit()
	p := w.cfgDir + "/usb-backup.recipients"
	data, _ := os.ReadFile(p)
	// An extra recipient added by someone else: refused.
	x, _ := age.GenerateHybridIdentity()
	os.WriteFile(p, append(append([]byte(nil), data...), []byte(x.Recipient().String()+"\n")...), 0o644)
	w.mustRun(7, "run")
	// Removed: with keys present this is not "no encryption" but exit 10.
	os.Remove(p)
	w.mustRun(10, "run")
	if exists(w.diskFile("BACKUP1", "Backup/2026")) {
		t.Fatal("something was written")
	}
	os.WriteFile(p, data, 0o644)
	w.mustRun(0, "run")
}
