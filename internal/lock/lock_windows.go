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

package lock

import (
	"errors"
	"fmt"
	"syscall"
	"unsafe"
)

var (
	kernel32        = syscall.NewLazyDLL("kernel32.dll")
	procCreateMutex = kernel32.NewProc("CreateMutexW")
)

const errorAlreadyExists = 183

// Acquire creates the named mutex Global\UsbBackup (path is the mutex name
// for tests; "" for the default). The mutex exists as long as its handle is
// open, so a second instance sees it already exists. The returned function
// closes the handle.
func Acquire(path string) (func(), error) {
	name := `Global\` + Name
	if path != "" {
		name = `Global\` + Name + "-" + fmt.Sprintf("%x", fnv(path))
	}
	p, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return nil, err
	}
	h, _, callErr := procCreateMutex.Call(0, 0, uintptr(unsafe.Pointer(p))) //nolint:gosec // Win32 call
	if h == 0 {
		return nil, fmt.Errorf("create mutex: %w", callErr)
	}
	if errors.Is(callErr, syscall.Errno(errorAlreadyExists)) {
		_ = syscall.CloseHandle(syscall.Handle(h))
		return nil, ErrBusy
	}
	return func() { _ = syscall.CloseHandle(syscall.Handle(h)) }, nil
}

func fnv(s string) uint32 {
	h := uint32(2166136261)
	for i := 0; i < len(s); i++ {
		h = (h ^ uint32(s[i])) * 16777619
	}
	return h
}

// DefaultPath returns the name of the mutex.
func DefaultPath() string { return `Global\` + Name }
