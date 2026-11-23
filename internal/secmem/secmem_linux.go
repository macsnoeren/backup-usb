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

package secmem

import "syscall"

func lock(b []byte) error   { return syscall.Mlock(b) }
func unlock(b []byte) error { return syscall.Munlock(b) }

const (
	prSetDumpable = 4
	prGetDumpable = 3
)

// ProtectProcess keeps secrets out of core dumps: the process is marked not
// dumpable (prctl PR_SET_DUMPABLE 0), which also stops other processes of
// the same user from attaching with ptrace.
func ProtectProcess() error {
	if _, _, errno := syscall.RawSyscall(syscall.SYS_PRCTL, prSetDumpable, 0, 0); errno != 0 {
		return errno
	}
	return nil
}

// dumpable reports the dumpable flag (for tests).
func dumpable() (bool, error) {
	r, _, errno := syscall.RawSyscall(syscall.SYS_PRCTL, prGetDumpable, 0, 0)
	if errno != 0 {
		return false, errno
	}
	return r != 0, nil
}
