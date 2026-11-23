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
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/macsnoeren/backup-usb/internal/app"
	"github.com/macsnoeren/backup-usb/internal/config"
)

func (w *world) keysCheck() app.KeysState {
	w.t.Helper()
	out, _ := w.mustRun(0, "keys", "check", "--json", "--vault", w.vaultPath())
	var st app.KeysState
	if err := json.Unmarshal([]byte(out), &st); err != nil {
		w.t.Fatalf("%v: %s", err, out)
	}
	return st
}

// keys check tells the window whether first-time setup is needed. It asks
// no password: there is none yet.
func TestKeysCheck(t *testing.T) {
	w := newWorld(t)
	if st := w.keysCheck(); st.State != app.KeysNone || !st.Writable || st.Vault != w.vaultPath() {
		t.Fatalf("fresh installation: %+v", st)
	}
	before, _ := os.ReadDir(w.cfgDir)
	w.keysCheck()
	if after, _ := os.ReadDir(w.cfgDir); len(after) != len(before) {
		t.Fatalf("keys check left files behind: %v", after)
	}
	w.keysInit()
	if st := w.keysCheck(); st.State != app.KeysReady {
		t.Fatalf("after keys init: %+v", st)
	}
	// Another Windows user, or a lost vault: never offer to make new keys.
	away := w.vaultPath() + ".away"
	if err := os.Rename(w.vaultPath(), away); err != nil {
		t.Fatal(err)
	}
	if st := w.keysCheck(); st.State != app.KeysNoVault {
		t.Fatalf("vault gone: %+v", st)
	}
	if err := os.Rename(away, w.vaultPath()); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(w.cfgDir, config.DefaultRecipients)); err != nil {
		t.Fatal(err)
	}
	if st := w.keysCheck(); st.State != app.KeysIncomplete {
		t.Fatalf("recipients gone: %+v", st)
	}
}

// Without administrator rights nothing may be left behind: a vault without
// recipients would block every later attempt with "keys already exist".
func TestKeysInitNotWritable(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs a folder this user cannot write")
	}
	w := newWorld(t)
	vault := filepath.Join(t.TempDir(), "recovery.vault")
	bg := filepath.Join(t.TempDir(), "bg.txt")
	if err := os.Chmod(w.cfgDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(w.cfgDir, 0o755) })
	out, _ := w.mustRun(0, "keys", "check", "--json", "--vault", vault)
	if !strings.Contains(out, `"writable": false`) {
		t.Fatalf("keys check: %s", out)
	}
	w.stdin = testPassword + "\n" + testPassword + "\n"
	_, errOut := w.mustRun(11, "keys", "init", "--vault", vault, "--breakglass-out", bg, "--password-stdin")
	if !strings.Contains(errOut, "administrator") {
		t.Errorf("no hint about administrator rights: %s", errOut)
	}
	if exists(vault) || exists(bg) {
		t.Fatal("a failed keys init left a vault or break-glass file")
	}
}

// A failure halfway removes what this run created, so it can be repeated.
func TestKeysInitRollsBack(t *testing.T) {
	w := newWorld(t)
	bg := filepath.Join(t.TempDir(), "bg.txt")
	// The signature cannot be written: a non-empty folder is in its place.
	block := filepath.Join(w.cfgDir, config.DefaultRecipients+".sig")
	if err := os.MkdirAll(filepath.Join(block, "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	w.stdin = testPassword + "\n" + testPassword + "\n"
	if code, _, _ := w.run("keys", "init", "--vault", w.vaultPath(), "--breakglass-out", bg, "--password-stdin"); code == 0 {
		t.Fatal("keys init succeeded")
	}
	if exists(w.vaultPath()) || exists(bg) || exists(filepath.Join(w.cfgDir, config.DefaultRecipients)) || exists(filepath.Join(w.cfgDir, config.SigningPubFile)) {
		t.Fatal("leftovers after a failed keys init")
	}
	if !exists(filepath.Join(block, "x")) {
		t.Fatal("rollback removed something it did not create")
	}
	if err := os.RemoveAll(block); err != nil {
		t.Fatal(err)
	}
	w.keysInit()
}
