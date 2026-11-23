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

package password

import (
	"fmt"
	"io"

	"github.com/macsnoeren/backup-usb/internal/secmem"
)

// Console asks on the terminal without echoing the input. Prompts go to
// Out (usually stderr).
type Console struct {
	Out io.Writer
}

// Password implements Source.
func (c Console) Password(prompt string) (*secmem.Buffer, error) {
	tty, restore, err := openTTYNoEcho()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tty.Close() }()
	fmt.Fprint(c.Out, prompt)
	buf, err := ReadLine(tty)
	restore()
	fmt.Fprintln(c.Out)
	return buf, err
}
