package lock

import (
	"bufio"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func TestLockRefusesSpecialFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.Symlink(filepath.Join(dir, "elsewhere"), filepath.Join(dir, "link.lock")); err == nil {
		if _, err := Acquire(filepath.Join(dir, "link.lock")); err == nil {
			t.Fatal("symlink lock file accepted")
		}
	}
	if err := mkfifo(filepath.Join(dir, "fifo.lock")); err == nil {
		done := make(chan error, 1)
		go func() { _, err := Acquire(filepath.Join(dir, "fifo.lock")); done <- err }()
		select {
		case err := <-done:
			if err == nil {
				t.Fatal("FIFO lock file accepted")
			}
		case <-time.After(5 * time.Second):
			t.Fatal("a FIFO at the lock path hangs Acquire")
		}
	}
	if p := DefaultPath(); p == "" || strings.HasPrefix(p, "/run/lock") {
		t.Fatalf("default lock path %q", p)
	}
}
