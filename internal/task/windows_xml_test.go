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
	"encoding/xml"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/macsnoeren/backup-usb/internal/testutil"
)

const exe = `C:\Program Files\Kantoor & Co\USB Backup\usb-backup.exe`

func TestWindowsXMLGolden(t *testing.T) {
	specs := map[string]Spec{
		"task-usb.xml": {Trigger: USB, Command: exe, Args: []string{"--auto"}},
	}
	for name, s := range specs {
		x, err := WindowsXML(s)
		if err != nil {
			t.Fatal(err)
		}
		testutil.Golden(t, name, []byte(x))
		// Well-formed XML, and the path comes back unchanged.
		var doc struct {
			Actions struct {
				Exec struct {
					Command   string
					Arguments string
				}
			}
		}
		if err := xml.Unmarshal([]byte(strings.Replace(x, `encoding="UTF-16"`, `encoding="UTF-8"`, 1)), &doc); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if doc.Actions.Exec.Command != `"`+s.Command+`"` {
			t.Fatalf("%s: command %q", name, doc.Actions.Exec.Command)
		}
		for _, want := range []string{"InteractiveToken", "LeastPrivilege", "IgnoreNew", "PT12H"} {
			if !strings.Contains(x, want) {
				t.Fatalf("%s lacks %s", name, want)
			}
		}
	}
}

// A window that stays next to the clock must not be ended after 12 hours.
func TestWindowsXMLStays(t *testing.T) {
	x, err := WindowsXML(Spec{Trigger: USB, Command: exe, Args: []string{"--auto"}, Stays: true})
	if err != nil || !strings.Contains(x, "<ExecutionTimeLimit>PT0S<") || strings.Contains(x, "PT12H") {
		t.Fatalf("%v\n%s", err, x)
	}
}

func TestValidate(t *testing.T) {
	bad := []Spec{
		{Trigger: OldDaily, Command: exe},
		{Trigger: OldWeekly, Command: exe},
		{Trigger: "hourly", Command: exe},
		{Trigger: USB},
		{Trigger: Login, Command: exe},
	}
	for _, s := range bad {
		if _, err := WindowsXML(s); err == nil {
			t.Errorf("%+v accepted", s)
		}
	}
}

func TestUTF16LE(t *testing.T) {
	b := UTF16LE("a\né")
	if b[0] != 0xff || b[1] != 0xfe {
		t.Fatal("no BOM")
	}
	u := make([]uint16, (len(b)-2)/2)
	for i := range u {
		u[i] = uint16(b[2+2*i]) | uint16(b[3+2*i])<<8
	}
	if string(utf16.Decode(u)) != "a\r\né" {
		t.Fatal(string(utf16.Decode(u)))
	}
}

func TestEscapeArg(t *testing.T) {
	for in, want := range map[string]string{"run": "run", "": `""`, `C:\Program Files\x`: `"C:\Program Files\x"`, `a"b c`: `"a\"b c"`, `dir with\`: `"dir with\\"`} {
		if got := escapeArg(in); got != want {
			t.Errorf("%q: %q, want %q", in, got, want)
		}
	}
}
