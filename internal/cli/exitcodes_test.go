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
	"fmt"
	"testing"

	"github.com/macsnoeren/backup-usb/internal/app"
)

func TestExitCodes(t *testing.T) {
	want := map[app.Kind]int{
		app.KindNone: 0, app.KindWarning: 1, app.KindNoDisk: 2, app.KindWrongDisk: 3, app.KindSkipped: 4,
		app.KindBusy: 5, app.KindUnknownDisk: 6, app.KindTampered: 7, app.KindDiskCheck: 10, app.KindIO: 11,
		app.KindVerify: 12, app.KindDecrypt: 13, app.KindUsage: 64,
	}
	for k, code := range want {
		var err error
		if k != app.KindNone {
			err = fmt.Errorf("wrapped: %w", app.Errorf(k, nil, "x"))
		}
		if got := ExitCode(err); got != code {
			t.Errorf("kind %d: exit %d, want %d", k, got, code)
		}
	}
	if ExitCode(errors.New("plain")) != ExitIO {
		t.Fatal("plain error should be an I/O error")
	}
}
