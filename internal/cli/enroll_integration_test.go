//go:build integration

package cli

import (
	"os"
	"strings"
	"testing"

	"github.com/macsnoeren/backup-usb/internal/disk"
)

// enroll registers a fake disk with the real enroll command.
func (w *world) enroll(name, weeks string) {
	w.t.Helper()
	w.stdin = testPassword + "\n"
	defer func() { w.stdin = "" }()
	w.mustRun(0, "enroll", w.disks.Disk(name).Mount, "--name", name, "--weeks", weeks, "--password-stdin", "--vault", w.vaultPath())
}

func TestEnroll(t *testing.T) {
	w := newWorld(t)
	w.stdin = testPassword + "\n"
	w.mustRun(64, "enroll", w.disks.Disk("BACKUP1").Mount, "--name", "BACKUP1", "--weeks", "odd", "--password-stdin")
	w.keysInit()
	w.enroll("BACKUP1", "odd")
	data, err := os.ReadFile(w.diskFile("BACKUP1", "Backup/disk-id.json"))
	if err != nil {
		t.Fatal(err)
	}
	id, err := disk.ParseIDFile(data)
	if err != nil || id.Name != "BACKUP1" || id.Signature == "" || id.VolumeSerial != w.disks.Disk("BACKUP1").VolumeSerial {
		t.Fatalf("ID file %s (%v)", data, err)
	}
	if st, _ := os.Stat(w.diskFile("BACKUP1", "Backup/disk-id.json")); st.Mode().Perm()&0o222 != 0 {
		t.Error("ID file is not read-only")
	}
	if !exists(w.cfgDir + "/usb-backup.disks.json.sig") {
		t.Fatal("registration list not signed")
	}
	// The disk is now recognised as BACKUP1 for odd weeks: week 47 runs.
	out, _ := w.mustRun(0, "run")
	if !strings.Contains(out, "BACKUP1") {
		t.Fatal(out)
	}
	// Registering the same name again needs --force; a wrong password 13.
	w.stdin = testPassword + "\n"
	w.mustRun(64, "enroll", w.disks.Disk("BACKUP2").Mount, "--name", "BACKUP1", "--weeks", "even", "--password-stdin", "--vault", w.vaultPath())
	w.stdin = "nope nope nope\n"
	w.mustRun(13, "enroll", w.disks.Disk("BACKUP2").Mount, "--name", "BACKUP2", "--weeks", "even", "--password-stdin", "--vault", w.vaultPath())
	w.enroll("BACKUP2", "even")
	// Re-enrolling a disk replaces its read-only ID file.
	w.stdin = testPassword + "\n"
	w.mustRun(0, "enroll", w.disks.Disk("BACKUP2").Mount, "--name", "BACKUP2", "--weeks", "even", "--force", "--password-stdin", "--vault", w.vaultPath())
	w.mustRun(64, "enroll", "nonexistent-disk", "--name", "X", "--weeks", "odd")
}
