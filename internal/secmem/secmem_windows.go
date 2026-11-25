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

import (
	"os"
	"syscall"
	"unsafe"
)

var (
	kernel32                      = syscall.NewLazyDLL("kernel32.dll")
	procVirtualLock               = kernel32.NewProc("VirtualLock")
	procVirtualUnlock             = kernel32.NewProc("VirtualUnlock")
	procSetErrorMode              = kernel32.NewProc("SetErrorMode")
	wer                           = syscall.NewLazyDLL("wer.dll")
	procWerAddExcludedApplication = wer.NewProc("WerAddExcludedApplication")
)

func lock(b []byte) error {
	if len(b) == 0 {
		return nil
	}
	r, _, err := procVirtualLock.Call(uintptr(unsafe.Pointer(&b[0])), uintptr(len(b))) //nolint:gosec // Win32 call
	if r == 0 {
		return err
	}
	return nil
}

func unlock(b []byte) error {
	if len(b) == 0 {
		return nil
	}
	r, _, err := procVirtualUnlock.Call(uintptr(unsafe.Pointer(&b[0])), uintptr(len(b))) //nolint:gosec // Win32 call
	if r == 0 {
		return err
	}
	return nil
}

const (
	semFailCriticalErrors = 0x0001
	semNoGPFaultErrorBox  = 0x0002
)

// ProtectProcess keeps secrets out of crash dumps: no Windows Error
// Reporting for this program (WerAddExcludedApplication) and no fault
// dialogs (SetErrorMode).
func ProtectProcess() error {
	_, _, _ = procSetErrorMode.Call(semFailCriticalErrors | semNoGPFaultErrorBox) // returns the old mode
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	name, err := syscall.UTF16PtrFromString(exe)
	if err != nil {
		return err
	}
	if err := procWerAddExcludedApplication.Find(); err != nil {
		return err
	}
	hr, _, _ := procWerAddExcludedApplication.Call(uintptr(unsafe.Pointer(name)), 0) //nolint:gosec // Win32 call
	if hr != 0 {
		return syscall.Errno(hr)
	}
	return nil
}

func dumpable() (bool, error) { return false, nil }
