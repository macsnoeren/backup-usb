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
	"os"
	"syscall"
)

var (
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	procSetConsoleMode = kernel32.NewProc("SetConsoleMode")
)

const enableEchoInput = 0x0004

// openTTYNoEcho opens the console input with echo switched off.
func openTTYNoEcho() (*os.File, func(), error) {
	name, _ := syscall.UTF16PtrFromString("CONIN$")
	h, err := syscall.CreateFile(name, syscall.GENERIC_READ|syscall.GENERIC_WRITE, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE, nil, syscall.OPEN_EXISTING, 0, 0)
	if err != nil {
		return nil, nil, ErrNoTTY
	}
	var mode uint32
	if err := syscall.GetConsoleMode(h, &mode); err != nil {
		_ = syscall.CloseHandle(h)
		return nil, nil, ErrNoTTY
	}
	if r, _, _ := procSetConsoleMode.Call(uintptr(h), uintptr(mode&^enableEchoInput)); r == 0 {
		_ = syscall.CloseHandle(h)
		return nil, nil, ErrNoTTY
	}
	f := os.NewFile(uintptr(h), "CONIN$")
	restore := func() { _, _, _ = procSetConsoleMode.Call(uintptr(h), uintptr(mode)) }
	return f, restore, nil
}
