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
	"os/exec"
	"testing"
)

// Removing a task that is not there is fine; a task that cannot be removed
// is an error.
func TestSchtasksUninstall(t *testing.T) {
	exit := func() error { return exec.Command("cmd.exe", "/c", "exit", "1").Run() }()
	var ee *exec.ExitError
	if !errors.As(exit, &ee) {
		t.Fatalf("no exit error: %v", exit)
	}
	for _, tc := range []struct {
		name       string
		del, query error
		ok         bool
	}{
		{"removed", nil, errors.New("not called"), true},
		{"was not there", exit, exit, true},
		{"cannot be removed", exit, nil, false},
		{"query did not run", exit, errors.New("schtasks: timed out"), false},
	} {
		s := &Schtasks{Run: func(args ...string) error {
			if args[0] == "/Query" {
				return tc.query
			}
			return tc.del
		}}
		if err := s.Uninstall(OldDaily); (err == nil) != tc.ok {
			t.Errorf("%s: %v", tc.name, err)
		}
	}
}
