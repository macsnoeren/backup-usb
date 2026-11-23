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
	"strings"
	"testing"

	"github.com/macsnoeren/backup-usb/internal/password"
)

func TestPasswordSource(t *testing.T) {
	c := &ctx{env: &Env{Stdin: strings.NewReader("from-the-pipe\n")}}
	b, err := c.passwordSource(true).Password("x")
	if err != nil || string(b.Bytes()) != "from-the-pipe" {
		t.Fatal(err)
	}
	if _, ok := c.passwordSource(false).(password.Console); !ok {
		t.Fatal("without --password-stdin the console is used")
	}
}
