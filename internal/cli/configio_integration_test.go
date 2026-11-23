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
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigSourcesAndImportExport(t *testing.T) {
	a := newWorld(t)
	a.keysInit()
	newSrc := t.TempDir()
	a.stdin = testPassword + "\n"
	a.mustRun(0, "config", "add-source", "--tag", "Photos", "--path", newSrc, "--exclude", "*.raw", "--password-stdin", "--vault", a.vaultPath())
	a.stdin = testPassword + "\n"
	a.mustRun(64, "config", "add-source", "--tag", "photos", "--path", newSrc, "--password-stdin", "--vault", a.vaultPath())
	a.stdin = testPassword + "\n"
	a.mustRun(0, "config", "remove-source", "Books", "--password-stdin", "--vault", a.vaultPath())
	a.stdin = testPassword + "\n"
	a.mustRun(0, "config", "set", "keepPerDisk", "8", "--password-stdin", "--vault", a.vaultPath())
	exported := filepath.Join(t.TempDir(), "settings.json")
	a.mustRun(0, "config", "export", exported)
	data, _ := os.ReadFile(exported)
	for _, secret := range []string{"AGE-SECRET-KEY", testPassword, "signing"} {
		if strings.Contains(string(data), secret) {
			t.Fatalf("export contains %q", secret)
		}
	}
	want, _ := a.mustRun(0, "config", "show")

	// Another machine with its own keys: import, signed with its key.
	b := newWorld(t)
	b.keysInit()
	b.stdin = testPassword + "\n"
	b.mustRun(0, "config", "import", exported, "--password-stdin", "--vault", b.vaultPath())
	got, _ := b.mustRun(0, "config", "show")
	if got != want {
		t.Fatalf("settings differ:\n%s\n---\n%s", got, want)
	}
	b.mustRun(2, "run", "--dry-run") // 2 (no disk), not 7: the signature is valid on b
	// A broken import file is refused and changes nothing.
	os.WriteFile(exported, []byte(`{"keepPerDisk": -1}`), 0o644)
	b.stdin = testPassword + "\n"
	b.mustRun(64, "config", "import", exported, "--password-stdin", "--vault", b.vaultPath())
	if again, _ := b.mustRun(0, "config", "show"); again != want {
		t.Fatal("failed import changed the settings")
	}
}
