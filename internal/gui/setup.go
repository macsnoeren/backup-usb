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
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/macsnoeren/backup-usb/internal/app"
)

// GearAction is what the gear button does.
type GearAction int

// Gear actions.
const (
	// GearLogin asks the vault password.
	GearLogin GearAction = iota
	// GearSetup starts the first-time setup: there are no keys at all, so
	// there is no password to ask.
	GearSetup
	// GearNeedAdmin explains that the first-time setup needs
	// administrator rights.
	GearNeedAdmin
	// GearLoginIncomplete warns that key files are missing, then asks the
	// vault password.
	GearLoginIncomplete
	// GearNoVault explains that keys exist but this user has no vault.
	GearNoVault
)

// ActionFor chooses the gear action. New keys are only offered when there
// are none; anything unexpected asks the password.
func ActionFor(st app.KeysState) GearAction {
	switch st.State {
	case app.KeysNone:
		if !st.Writable {
			return GearNeedAdmin
		}
		return GearSetup
	case app.KeysNoVault:
		return GearNoVault
	case app.KeysIncomplete:
		return GearLoginIncomplete
	default:
		return GearLogin
	}
}

// KeysState asks the CLI which key files exist (no password needed).
func (c CLI) KeysState(ctx context.Context) (app.KeysState, error) {
	ev := c.Call(ctx, nil, "keys", "check", "--json")
	if ev.Event != "result" {
		return app.KeysState{}, errors.New(ev.Message)
	}
	var st app.KeysState
	if err := json.Unmarshal([]byte(ev.Output), &st); err != nil || st.State == "" {
		return app.KeysState{}, fmt.Errorf("unexpected answer from usb-backup-cli: %q", ev.Output)
	}
	return st, nil
}

// IsNewFile reports whether nothing exists at path. The break-glass key
// only goes to a new file: an existing one may be the only copy of the
// current key, so it is never removed or replaced.
func IsNewFile(path string) bool {
	_, err := os.Lstat(path)
	return errors.Is(err, fs.ErrNotExist)
}
