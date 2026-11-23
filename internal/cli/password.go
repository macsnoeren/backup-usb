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
	"github.com/macsnoeren/backup-usb/internal/password"
)

// passwordSource returns where passwords come from: stdin with
// --password-stdin (also the pipe from the GUI), otherwise the console
// without echo. Never from arguments or environment variables.
func (c *ctx) passwordSource(fromStdin bool) password.Source {
	// With --json-stream the GUI sends the password as the first line on
	// stdin.
	if fromStdin || c.stream != nil {
		return password.Reader{R: c.env.Stdin}
	}
	if c.env.Console != nil {
		return c.env.Console
	}
	return password.Console{Out: c.env.Stderr}
}
