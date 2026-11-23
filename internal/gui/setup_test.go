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

package gui

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/macsnoeren/backup-usb/internal/app"
)

func TestGearAction(t *testing.T) {
	for _, tc := range []struct {
		st   app.KeysState
		want GearAction
	}{
		{app.KeysState{State: app.KeysNone, Writable: true}, GearSetup},
		{app.KeysState{State: app.KeysNone}, GearNeedAdmin},
		{app.KeysState{State: app.KeysReady}, GearLogin},
		{app.KeysState{State: app.KeysReady, Writable: true}, GearLogin},
		{app.KeysState{State: app.KeysIncomplete, Writable: true}, GearLoginIncomplete},
		// Keys exist: never offer to make new ones.
		{app.KeysState{State: app.KeysNoVault, Writable: true}, GearNoVault},
		{app.KeysState{State: "something new", Writable: true}, GearLogin},
	} {
		if got := ActionFor(tc.st); got != tc.want {
			t.Errorf("%+v: %v, want %v", tc.st, got, tc.want)
		}
	}
}

func TestKeysStateCall(t *testing.T) {
	c := fakeCLI(t, "stream", `FAKE_LINES={"event":"result","code":0,"output":"{\"state\":\"none\",\"vault\":\"v\",\"writable\":true}"}`)
	st, err := c.KeysState(context.Background())
	if err != nil || st.State != app.KeysNone || !st.Writable || st.Vault != "v" {
		t.Fatalf("%+v %v", st, err)
	}
	c = fakeCLI(t, "stream", `FAKE_LINES={"event":"error","code":7,"message":"settings changed"}`)
	if _, err := c.KeysState(context.Background()); err == nil || err.Error() != "settings changed" {
		t.Fatalf("err = %v", err)
	}
	c = fakeCLI(t, "stream", `FAKE_LINES={"event":"result","code":0,"output":"nonsense"}`)
	if _, err := c.KeysState(context.Background()); err == nil {
		t.Fatal("nonsense accepted as a key state")
	}
}

// The break-glass file is never an existing file: it could be the only
// copy of the current break-glass key.
func TestIsNewFile(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "old.txt")
	if err := os.WriteFile(old, []byte("key"), 0o600); err != nil {
		t.Fatal(err)
	}
	if IsNewFile(old) || !IsNewFile(filepath.Join(dir, "new.txt")) {
		t.Fatal("IsNewFile")
	}
}
