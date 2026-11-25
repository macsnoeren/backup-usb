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
	"bytes"
	"encoding/binary"
	"encoding/xml"
	"strings"
	"unicode/utf16"
)

const usbSubscription = `<QueryList><Query Id="0" Path="Microsoft-Windows-Partition/Diagnostic"><Select Path="Microsoft-Windows-Partition/Diagnostic">*[System[(EventID=1006)]]</Select></Query></QueryList>`

func esc(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

// windowsArgs joins arguments with Windows quoting rules.
func windowsArgs(args []string) string {
	q := make([]string, len(args))
	for i, a := range args {
		q[i] = escapeArg(a)
	}
	return strings.Join(q, " ")
}

// WindowsXML returns the Task Scheduler definition for spec: the user's
// interactive session (so the window and messages are visible), least
// privilege, a new start is ignored while one runs, no battery limits, at
// most 12 hours unless the program stays.
func WindowsXML(s Spec) (string, error) {
	if err := s.Validate(); err != nil {
		return "", err
	}
	if s.Trigger != USB {
		return "", errTrigger(s.Trigger)
	}
	trig := "    <EventTrigger>\n      <Enabled>true</Enabled>\n      <Subscription>" + esc(usbSubscription) + "</Subscription>\n    </EventTrigger>\n"
	limit := "PT12H"
	if s.Stays {
		limit = "PT0S" // none
	}
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-16"?>` + "\n")
	b.WriteString(`<Task version="1.2" xmlns="http://schemas.microsoft.com/windows/2004/02/mit/task">` + "\n")
	b.WriteString("  <RegistrationInfo>\n    <Description>" + esc("USB Backup: "+string(s.Trigger)+" trigger") + "</Description>\n    <URI>" + esc(`\`+Name(s.Trigger)) + "</URI>\n  </RegistrationInfo>\n")
	b.WriteString("  <Triggers>\n" + trig + "  </Triggers>\n")
	b.WriteString("  <Principals>\n    <Principal id=\"Author\">\n      <LogonType>InteractiveToken</LogonType>\n      <RunLevel>LeastPrivilege</RunLevel>\n    </Principal>\n  </Principals>\n")
	b.WriteString("  <Settings>\n    <MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy>\n    <DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries>\n    <StopIfGoingOnBatteries>false</StopIfGoingOnBatteries>\n    <StartWhenAvailable>false</StartWhenAvailable>\n    <ExecutionTimeLimit>" + limit + "</ExecutionTimeLimit>\n    <Enabled>true</Enabled>\n    <Hidden>false</Hidden>\n  </Settings>\n")
	b.WriteString("  <Actions Context=\"Author\">\n    <Exec>\n      <Command>" + esc(escapeArg(s.Command)) + "</Command>\n")
	if len(s.Args) > 0 {
		b.WriteString("      <Arguments>" + esc(windowsArgs(s.Args)) + "</Arguments>\n")
	}
	b.WriteString("    </Exec>\n  </Actions>\n</Task>\n")
	return b.String(), nil
}

// UTF16LE encodes s as UTF-16 little endian with a byte order mark, as
// schtasks /XML expects. Line endings become CRLF.
func UTF16LE(s string) []byte {
	s = strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\n", "\r\n")
	u := utf16.Encode([]rune(s))
	out := make([]byte, 2+2*len(u))
	out[0], out[1] = 0xff, 0xfe
	for i, c := range u {
		binary.LittleEndian.PutUint16(out[2+2*i:], c)
	}
	return out
}

// escapeArg quotes an argument with the rules of CommandLineToArgvW (the
// same as syscall.EscapeArg on Windows).
func escapeArg(s string) string {
	if s == "" {
		return `""`
	}
	if !strings.ContainsAny(s, " \t\"") {
		return s
	}
	var b strings.Builder
	b.WriteByte('"')
	slashes := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '\\':
			slashes++
		case '"':
			for ; slashes > 0; slashes-- {
				b.WriteByte('\\')
			}
			b.WriteByte('\\')
		default:
			slashes = 0
		}
		b.WriteByte(c)
	}
	for ; slashes > 0; slashes-- {
		b.WriteByte('\\')
	}
	b.WriteByte('"')
	return b.String()
}
