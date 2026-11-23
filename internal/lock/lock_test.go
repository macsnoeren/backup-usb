package lock

import (
	"bufio"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	_ "github.com/macsnoeren/backup-usb/internal/testutil" // -update flag
)

// The test binary doubles as the second process.
func TestMain(m *testing.M) {
	if p := os.Getenv("USB_BACKUP_LOCK_HELPER"); p != "" {
		release, err := Acquire(p)
		if err != nil {
			os.Stdout.WriteString("error " + err.Error() + "\n")
			os.Exit(1)
		}
		os.Stdout.WriteString("locked\n")
		_, _ = io.Copy(io.Discard, os.Stdin) // hold until the parent closes stdin
		release()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestTwoProcesses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "usb-backup.lock")
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	cmd.Env = append(os.Environ(), "USB_BACKUP_LOCK_HELPER="+path)
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	line, _ := bufio.NewReader(stdout).ReadString('\n')
	if line != "locked\n" {
		t.Fatalf("helper: %q", line)
	}
	if _, err := Acquire(path); !errors.Is(err, ErrBusy) {
		t.Fatalf("second instance got the lock: %v", err)
	}
	stdin.Close()
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
	release, err := Acquire(path)
	if err != nil {
		t.Fatalf("lock not free after the first process ended: %v", err)
	}
	if _, err := Acquire(path); !errors.Is(err, ErrBusy) {
		t.Fatal("lock taken twice in one process")
	}
	release()
	release2, err := Acquire(path)
	if err != nil {
		t.Fatal(err)
	}
	release2()
}
