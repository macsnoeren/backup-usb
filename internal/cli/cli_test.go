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

package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/macsnoeren/backup-usb/internal/secmem"
	"github.com/macsnoeren/backup-usb/internal/testutil"
)

// run calls the CLI in-process with a test environment.
func run(t *testing.T, env *Env, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	if env == nil {
		env = &Env{Now: time.Now, Getenv: func(string) string { return "" }}
	}
	env.Stdout, env.Stderr = &out, &errOut
	if env.Stdin == nil {
		env.Stdin = strings.NewReader("")
	}
	if env.Console == nil {
		env.Console = noConsole{} // never wait for somebody at the real one
	}
	code = env.Run(append([]string{"usb-backup-cli"}, args...))
	return code, out.String(), errOut.String()
}

// noConsole is a program that runs without a terminal.
type noConsole struct{}

func (noConsole) Password(string) (*secmem.Buffer, error) {
	return nil, errors.New("no terminal")
}

func TestRunWithoutProcess(t *testing.T) {
	var out, errOut bytes.Buffer
	code := Run([]string{"usb-backup-cli", "version"}, strings.NewReader(""), &out, &errOut)
	if code != 0 || !strings.Contains(out.String(), "version") {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, out.String(), errOut.String())
	}
}

func TestVersionHelpUnknown(t *testing.T) {
	Version = "1.2.3"
	tests := []struct {
		args     []string
		code     int
		inStdout string
		inStderr string
	}{
		{[]string{"version"}, 0, "1.2.3", ""},
		{[]string{"version", "--json"}, 0, `"version": "1.2.3"`, ""},
		{[]string{"version"}, 0, "GNU GPL version 3 or later", ""},
		{[]string{"version", "--json"}, 0, `"license": "GPL-3.0-or-later"`, ""},
		{[]string{"--version"}, 0, "1.2.3", ""},
		{[]string{"help"}, 0, "Usage: usb-backup-cli", ""},
		{nil, 0, "Usage: usb-backup-cli", ""},
		{[]string{"help", "version"}, 0, "-json", ""},
		{[]string{"version", "-h"}, 0, "", "-json"},
		{[]string{"bogus"}, 64, "", "unknown command"},
		{[]string{"help", "bogus"}, 64, "", "unknown command"},
		{[]string{"version", "--nope"}, 64, "", "not defined"},
		{[]string{"version", "extra"}, 64, "", "unexpected argument"},
		{[]string{"version", "--password", "x"}, 64, "", "never be passed as an argument"},
		{[]string{"version", "--password=x"}, 64, "", "never be passed as an argument"},
	}
	for _, tt := range tests {
		code, out, errOut := run(t, nil, tt.args...)
		if code != tt.code || !strings.Contains(out, tt.inStdout) || !strings.Contains(errOut, tt.inStderr) {
			t.Errorf("%v: exit %d (want %d)\nstdout %q\nstderr %q", tt.args, code, tt.code, out, errOut)
		}
		if strings.Contains(errOut, "x\n") && strings.Contains(strings.Join(tt.args, " "), "password") {
			t.Errorf("%v: password value echoed", tt.args)
		}
	}
	_ = testutil.Update
}

func TestEveryCommandHasHelp(t *testing.T) {
	for _, name := range commandNames() {
		code, out, _ := run(t, nil, "help", name)
		if code != 0 || !strings.Contains(out, name) {
			t.Errorf("help %s: exit %d, %q", name, code, out)
		}
	}
}
