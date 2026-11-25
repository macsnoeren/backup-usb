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
	"strings"
)

// ErrNotProgramFiles: what starts for every user must not be replaceable by
// one of them.
var ErrNotProgramFiles = errors.New("only a program in Program Files can start at login, because it starts for every user")

// underFolder reports whether a Windows path is below folder.
func underFolder(p, folder string) bool {
	norm := func(s string) string { return strings.ToLower(strings.ReplaceAll(s, "/", `\`)) }
	p, folder = norm(p), strings.TrimRight(norm(folder), `\`)
	if folder == "" || !strings.HasPrefix(p, folder+`\`) {
		return false
	}
	for _, part := range strings.Split(p, `\`) {
		if part == ".." || part == "." {
			return false
		}
	}
	return true
}

// loginCommand is the command line for a Login spec. The program is always
// quoted: without quotes Windows would try every part of a path with spaces.
func loginCommand(s Spec) (string, error) {
	if err := s.Validate(); err != nil {
		return "", err
	}
	if s.Trigger != Login {
		return "", errTrigger(s.Trigger)
	}
	for _, a := range append([]string{s.Command}, s.Args...) {
		if strings.ContainsFunc(a, func(r rune) bool { return r < 0x20 || r == 0x7f }) {
			return "", fmt.Errorf("control character in %q", a)
		}
	}
	if strings.Contains(s.Command, `"`) {
		return "", fmt.Errorf("invalid program path %q", s.Command)
	}
	cmd := `"` + s.Command + `"`
	if len(s.Args) > 0 {
		cmd += " " + windowsArgs(s.Args)
	}
	return cmd, nil
}
