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

//go:build !windows

// Package gui contains the Windows user interface. On other platforms only a
// stub exists.
package gui

import (
	"fmt"
	"os"
)

// Version is the program version, set by main.
var Version = "dev"

// Main reports that the GUI is only available on Windows.
func Main(_ []string) int {
	fmt.Fprintln(os.Stderr, "usb-backup: the graphical interface is only available on Windows; use usb-backup-cli")
	return 64
}
