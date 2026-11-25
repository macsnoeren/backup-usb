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
	"fmt"
	"os"
	"syscall"
	"testing"
	"unsafe"
)

// The key of the user is used: no administrator rights, and nothing starts.
func TestRunKey(t *testing.T) {
	path := fmt.Sprintf(`Software\usb-backup-test-%d`, os.Getpid())
	k := &runKey{root: syscall.HKEY_CURRENT_USER, path: path, anyProgram: true}
	t.Cleanup(func() {
		p, _ := syscall.UTF16PtrFromString(path)
		advapi32.NewProc("RegDeleteKeyW").Call(uintptr(syscall.HKEY_CURRENT_USER), uintptr(unsafe.Pointer(p)))
	})
	s := &Schtasks{login: k, Run: func(...string) error { t.Fatal("schtasks used"); return nil }}
	if s.Installed(Login) {
		t.Fatal("registered at the start")
	}
	if err := s.Uninstall(Login); err != nil {
		t.Fatalf("remove what is not there: %v", err)
	}
	k.anyProgram = false
	if err := s.Install(Spec{Trigger: Login, Command: `C:\USB-Backup\usb-backup.exe`}); !errors.Is(err, ErrNotProgramFiles) || s.Installed(Login) {
		t.Fatalf("outside Program Files: %v", err)
	}
	if pf := programFiles(); !underFolder(pf+`\x.exe`, os.Getenv("ProgramW6432")) {
		t.Fatalf("Program Files: %q", pf)
	}
	k.anyProgram = true
	for range 2 { // again is fine
		if err := s.Install(Spec{Trigger: Login, Command: exe, Args: []string{"--tray"}}); err != nil {
			t.Fatal(err)
		}
	}
	if !s.Installed(Login) {
		t.Fatal("not registered")
	}
	if err := s.Uninstall(Login); err != nil || s.Installed(Login) {
		t.Fatalf("not removed: %v", err)
	}
}
