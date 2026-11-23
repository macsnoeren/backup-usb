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
	"sort"
	"strings"
	"testing"

	"github.com/macsnoeren/backup-usb/internal/testutil"
)

func TestSystemdGolden(t *testing.T) {
	specs := map[string]Spec{
		"usb":    {Trigger: USB, Command: "/usr/bin/usb-backup-cli", Args: []string{"run", "--quiet-skip", "--wait", "30"}, VolumeUUIDs: []string{"A41C-77E2", "0f3c1e2a-9b8d-4c7e-a6f5-112233445566"}},
		"daily":  {Trigger: Daily, At: "02:00", Command: "/usr/bin/usb-backup-cli", Args: []string{"run", "--quiet-skip", "--config", "/etc/usb backup"}},
		"weekly": {Trigger: Weekly, At: "18:00", Day: "fri", Command: "/usr/bin/usb-backup-cli", Args: []string{"run", "--quiet-skip"}},
	}
	for name, s := range specs {
		files, err := SystemdFiles(s)
		if err != nil {
			t.Fatal(err)
		}
		var names []string
		for n := range files {
			names = append(names, n)
		}
		sort.Strings(names)
		var all strings.Builder
		for _, n := range names {
			all.WriteString("### " + n + "\n" + files[n] + "\n")
		}
		testutil.Golden(t, "systemd-"+name+".txt", []byte(all.String()))
	}
	if _, err := SystemdFiles(Spec{Trigger: USB, Command: "/x", VolumeUUIDs: []string{`x", RUN+="/bin/sh`}}); err == nil {
		t.Fatal("udev injection accepted")
	}
	if _, err := SystemdFiles(Spec{Trigger: USB, Command: "/x"}); err == nil {
		t.Fatal("usb rule without disks")
	}
	if _, err := SystemdFiles(Spec{Trigger: Daily, At: "02:00", Command: "/usr/bin/x", Args: []string{"--config", "/x\nExecStartPre=/bin/sh -c id"}}); err == nil {
		t.Fatal("newline injection accepted")
	}
	if got := systemdQuote(`a "b" $c 50%`); got != `"a \"b\" $$c 50%%"` {
		t.Fatal(got)
	}
}
