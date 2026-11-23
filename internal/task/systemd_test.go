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
