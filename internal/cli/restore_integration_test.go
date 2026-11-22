//go:build integration

package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/macsnoeren/backup-usb/internal/app"
	"github.com/macsnoeren/backup-usb/internal/testutil"
)

func TestRestoreSingleFiles(t *testing.T) {
	w := newWorld(t)
	w.enrollUnsigned(map[string]string{"BACKUP1": "odd"})
	big := strings.Repeat("0123456789", 200000)
	testutil.WriteFile(t, w.srcDir, "Data/zzz/last-in-the-zip.bin", big)
	bg := w.keysInit()
	w.mustRun(0, "run")

	to := t.TempDir()
	out, _ := w.mustRun(0, "restore", "--tag", "Data", "--week", "2026-47", "--file", "zzz/last-in-the-zip.bin", "--to", to, "--identity", bg)
	if !strings.Contains(out, "Restored 1 files") {
		t.Fatalf("%s", out)
	}
	got, err := os.ReadFile(filepath.Join(to, "zzz", "last-in-the-zip.bin"))
	if err != nil || string(got) != big {
		t.Fatal("restored file differs", err)
	}
	// A folder restores everything below it; times are kept.
	to2 := t.TempDir()
	w.mustRun(0, "restore", "--tag", "Data", "--file", "notes", "--to", to2, "--identity", bg)
	tree := testutil.Tree(t, to2)
	if tree["notes/todo.txt"] != "buy milk" || len(tree) != 2 {
		t.Fatalf("tree %v", tree)
	}
	// Whole source, with the vault password.
	to3 := t.TempDir()
	w.stdin = testPassword + "\n"
	w.mustRun(0, "restore", "--tag", "Books", "--to", to3, "--password-stdin", "--vault", w.vaultPath())
	if testutil.Tree(t, to3)["2026/q1.xlsx"] != "numbers" {
		t.Fatal("whole source")
	}
	w.mustRun(64, "restore", "--tag", "Data", "--file", "no/such/file", "--to", t.TempDir(), "--identity", bg)
	w.mustRun(64, "restore", "--tag", "Data", "--identity", bg)
}

func TestRestoreChecksHash(t *testing.T) {
	w := newWorld(t)
	w.enrollUnsigned(map[string]string{"BACKUP1": "odd"})
	w.mustRun(0, "run")
	// Damage the unencrypted index: a wrong SHA-256 for todo.txt.
	root, _ := os.OpenRoot(w.diskFile("BACKUP1", "Backup"))
	ix, err := app.ReadIndex(root, "2026/47", "Data", nil)
	root.Close()
	if err != nil {
		t.Fatal(err)
	}
	for i := range ix.Entries {
		if ix.Entries[i].Path == "notes/todo.txt" {
			ix.Entries[i].SHA256 = strings.Repeat("0", 64)
		}
	}
	f, _ := os.Create(w.diskFile("BACKUP1", "Backup/2026/47/Data.index.gz"))
	ix.Encode(f)
	f.Close()
	to := t.TempDir()
	_, errOut := w.mustRun(12, "restore", "--tag", "Data", "--file", "notes/todo.txt", "--to", to)
	if !strings.Contains(errOut, "does not match its SHA-256") {
		t.Fatalf("stderr %q", errOut)
	}
	if tree := testutil.Tree(t, to); tree["notes/todo.txt"] != "" || len(tree) > 1 {
		t.Fatalf("a file that failed the check was kept: %v", tree)
	}
}

func TestRestoreSymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks")
	}
	w := newWorld(t)
	w.enrollUnsigned(map[string]string{"BACKUP1": "odd"})
	os.Symlink("notes/todo.txt", filepath.Join(w.srcDir, "Data", "inside"))
	os.Symlink("/etc/passwd", filepath.Join(w.srcDir, "Data", "outside"))
	os.Symlink("../../escape", filepath.Join(w.srcDir, "Data", "notes", "up"))
	w.mustRun(0, "run")
	to := t.TempDir()
	out, _ := w.mustRun(0, "restore", "--tag", "Data", "--to", to)
	if target, err := os.Readlink(filepath.Join(to, "inside")); err != nil || target != "notes/todo.txt" {
		t.Fatalf("inside link: %v %v", target, err)
	}
	for _, l := range []string{"outside", "notes/up"} {
		if _, err := os.Lstat(filepath.Join(to, l)); err == nil {
			t.Fatalf("link %s pointing outside was restored", l)
		}
	}
	if strings.Count(out, "points outside") != 2 {
		t.Fatalf("%s", out)
	}
}
