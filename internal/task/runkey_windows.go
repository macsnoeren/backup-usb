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

package task

import (
	"errors"
	"path/filepath"
	"syscall"
	"unsafe"
)

// runKey is a registry key with programs that Windows starts at login.
type runKey struct {
	root syscall.Handle
	path string
	// anyProgram skips the check of the folder (tests).
	anyProgram bool
}

// machineRun is for all users; only administrators can change it.
var machineRun = runKey{root: syscall.HKEY_LOCAL_MACHINE, path: `Software\Microsoft\Windows\CurrentVersion\Run`}

const maxRunValue = 260 // Windows does not start longer command lines

var (
	advapi32           = syscall.NewLazyDLL(filepath.Join(systemDir(), "advapi32.dll"))
	procRegCreateKeyEx = advapi32.NewProc("RegCreateKeyExW")
	procRegSetValueEx  = advapi32.NewProc("RegSetValueExW")
	procRegDeleteValue = advapi32.NewProc("RegDeleteValueW")
)

// systemDir is the folder of the system DLLs, asked from Windows and not
// from the environment.
func systemDir() string {
	buf := make([]uint16, syscall.MAX_PATH)
	p := syscall.NewLazyDLL("kernel32.dll").NewProc("GetSystemDirectoryW")
	n, _, _ := p.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf))) //nolint:gosec // Win32 call
	if n == 0 || n >= uintptr(len(buf)) {
		return `C:\Windows\System32`
	}
	return syscall.UTF16ToString(buf[:n])
}

// programFiles is the Program Files folder, from a key that only
// administrators can change (the environment can be changed by the user);
// "" if it cannot be read.
func programFiles() string {
	k := &runKey{root: syscall.HKEY_LOCAL_MACHINE, path: `Software\Microsoft\Windows\CurrentVersion`}
	h, err := k.open(syscall.KEY_QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer func() { _ = syscall.RegCloseKey(h) }() // read-only
	n, err := syscall.UTF16PtrFromString("ProgramFilesDir")
	if err != nil {
		return ""
	}
	var buf [syscall.MAX_PATH]uint16
	size, kind := uint32(unsafe.Sizeof(buf)), uint32(0)
	if err := syscall.RegQueryValueEx(h, n, nil, &kind, (*byte)(unsafe.Pointer(&buf[0])), &size); err != nil || kind != syscall.REG_SZ { //nolint:gosec // Win32 call
		return ""
	}
	return syscall.UTF16ToString(buf[:])
}

func (k *runKey) open(access uint32) (syscall.Handle, error) {
	p, err := syscall.UTF16PtrFromString(k.path)
	if err != nil {
		return 0, err
	}
	var h syscall.Handle
	err = syscall.RegOpenKeyEx(k.root, p, 0, access|syscall.KEY_WOW64_64KEY, &h)
	return h, err
}

// set registers the program of spec under name.
func (k *runKey) set(name string, spec Spec) error {
	cmd, err := loginCommand(spec)
	if err != nil {
		return err
	}
	if !k.anyProgram && !underFolder(spec.Command, programFiles()) {
		return ErrNotProgramFiles
	}
	n, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return err
	}
	data, err := syscall.UTF16FromString(cmd)
	if err != nil {
		return err
	}
	if len(data) > maxRunValue {
		return errors.New("the path of the program is too long to start at login")
	}
	p, err := syscall.UTF16PtrFromString(k.path)
	if err != nil {
		return err
	}
	var h syscall.Handle
	r, _, _ := procRegCreateKeyEx.Call(uintptr(k.root), uintptr(unsafe.Pointer(p)), 0, 0, 0, //nolint:gosec // Win32 call
		uintptr(syscall.KEY_SET_VALUE|syscall.KEY_WOW64_64KEY), 0, uintptr(unsafe.Pointer(&h)), 0) //nolint:gosec // Win32 call
	if r != 0 {
		return syscall.Errno(r)
	}
	r, _, _ = procRegSetValueEx.Call(uintptr(h), uintptr(unsafe.Pointer(n)), 0, syscall.REG_SZ, //nolint:gosec // Win32 call
		uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)*2)) //nolint:gosec // Win32 call
	cerr := syscall.RegCloseKey(h)
	if r != 0 {
		return syscall.Errno(r)
	}
	return cerr
}

// remove removes name; a name that is not there is fine.
func (k *runKey) remove(name string) error {
	if !k.exists(name) {
		return nil // no administrator rights needed
	}
	n, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return err
	}
	h, err := k.open(syscall.KEY_SET_VALUE)
	if err != nil {
		return err
	}
	r, _, _ := procRegDeleteValue.Call(uintptr(h), uintptr(unsafe.Pointer(n))) //nolint:gosec // Win32 call
	cerr := syscall.RegCloseKey(h)
	if r != 0 && syscall.Errno(r) != syscall.ERROR_FILE_NOT_FOUND {
		return syscall.Errno(r)
	}
	return cerr
}

// exists reports whether name is registered.
func (k *runKey) exists(name string) bool {
	n, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return false
	}
	h, err := k.open(syscall.KEY_QUERY_VALUE)
	if err != nil {
		return false
	}
	defer func() { _ = syscall.RegCloseKey(h) }() // read-only
	return syscall.RegQueryValueEx(h, n, nil, nil, nil, nil) == nil
}
