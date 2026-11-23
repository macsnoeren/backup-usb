package task

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSystemdInstallUninstall(t *testing.T) {
	var calls []string
	s := &Systemd{UnitDir: t.TempDir(), RulesDir: t.TempDir(), Run: func(name string, args ...string) error {
		calls = append(calls, name+" "+strings.Join(args, " "))
		return nil
	}}
	spec := Spec{Trigger: Daily, At: "02:00", Command: "/usr/bin/usb-backup-cli", Args: []string{"run", "--quiet-skip"}}
	if err := s.Install(spec); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(s.UnitDir, "usb-backup-daily.timer")); err != nil {
		t.Fatal(err)
	}
	if strings.Join(calls, ";") != "systemctl daemon-reload;systemctl enable --now usb-backup-daily.timer" {
		t.Fatal(calls)
	}
	if err := s.Install(Spec{Trigger: USB, Command: "/usr/bin/usb-backup-cli", VolumeUUIDs: []string{"A41C-77E2"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(s.RulesDir, UdevRule)); err != nil {
		t.Fatal(err)
	}
	for _, tr := range Triggers {
		if err := s.Uninstall(tr); err != nil {
			t.Fatal(err)
		}
	}
	for _, d := range []string{s.UnitDir, s.RulesDir} {
		if e, _ := os.ReadDir(d); len(e) != 0 {
			t.Fatalf("left in %s: %v", d, e)
		}
	}
}
