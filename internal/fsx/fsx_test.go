package fsx

import (
	"os"
	"path/filepath"
	"testing"

	_ "github.com/macsnoeren/backup-usb/internal/testutil" // -update flag
)

func TestWriteFileAtomic(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x.txt")
	if err := WriteFileAtomic(p, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(p)
	if string(data) != "hello" {
		t.Fatal(string(data))
	}
	if _, err := os.Stat(p + PartSuffix); !os.IsNotExist(err) {
		t.Fatal("part file left behind")
	}
	if err := WriteFileAtomic(filepath.Join(dir, "missing", "x"), nil, 0o600); err == nil {
		t.Fatal("expected error")
	}
}

func TestSafeJoin(t *testing.T) {
	base := t.TempDir()
	good := []string{"a", "a/b.txt", "dir/", "naïve/€.txt", "a..b", ".hidden"}
	for _, g := range good {
		if _, err := SafeJoin(base, g); err != nil {
			t.Errorf("%q rejected: %v", g, err)
		}
	}
	everywhere := []string{"", "/etc/passwd", "../x", "a/../../x", "a/./b", "a//b", "//server/share", "a\x00b", "a\nb", "..", ".", "evil\u202Etxt.exe", "bad\xffutf8"}
	windowsOnly := []string{`a\b`, `..\x`, "C:/x", "c:x", "a:stream", "CON", "dir/nul.txt", "LPT1", "COM\u00b9", "lpt\u00b3.txt", "foo.", "foo ", "a/.. /b", "a<b", "q?", "x|y", `"x"`, "st*r"}
	for _, b := range everywhere {
		for _, goos := range []string{"linux", "windows"} {
			if _, err := SafeRelOS(b, goos); err == nil {
				t.Errorf("%s: %q accepted", goos, b)
			}
		}
	}
	for _, b := range windowsOnly {
		if _, err := SafeRelOS(b, "windows"); err == nil {
			t.Errorf("windows: %q accepted", b)
		}
	}
	for _, ok := range []string{"2024-01-01 10:00.txt", `back\slash`} {
		if _, err := SafeRelOS(ok, "linux"); err != nil {
			t.Errorf("linux: %q rejected: %v", ok, err)
		}
	}
}

func FuzzSafeJoin(f *testing.F) {
	f.Add("a/b")
	f.Add("../x")
	f.Fuzz(func(t *testing.T, name string) {
		base := "/base/dir"
		p, err := SafeJoin(base, name)
		if err != nil {
			return
		}
		rel, rerr := filepath.Rel(base, p)
		if rerr != nil || rel == ".." || len(rel) >= 3 && rel[:3] == "../" || filepath.IsAbs(rel) {
			t.Fatalf("%q escapes: %q", name, p)
		}
	})
}

func TestDiskRootBlocksSymlinks(t *testing.T) {
	outside := t.TempDir()
	victim := filepath.Join(outside, "victim.txt")
	if err := os.WriteFile(victim, []byte("precious"), 0o600); err != nil {
		t.Fatal(err)
	}
	mount := t.TempDir()
	if err := os.MkdirAll(filepath.Join(mount, "Backup", "2026"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(victim, filepath.Join(mount, "Backup", "2026", "Data.zip.part")); err != nil {
		t.Skip("no symlinks:", err)
	}
	if err := os.Symlink(outside, filepath.Join(mount, "Backup", "47")); err != nil {
		t.Fatal(err)
	}
	root, err := OpenDiskRoot(mount, "Backup")
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	f, err := CreateExclusive(root, "2026/Data.zip.part", 0o600)
	if err != nil {
		t.Fatal(err)
	}
	f.Write([]byte("zip"))
	f.Close()
	if data, _ := os.ReadFile(victim); string(data) != "precious" {
		t.Fatal("file outside the disk was overwritten")
	}
	if err := RootWriteFileAtomic(root, "47/x.txt", []byte("x"), 0o600); err == nil {
		t.Fatal("wrote through a symlinked folder")
	}
	if _, err := os.Stat(filepath.Join(outside, "x.txt")); err == nil {
		t.Fatal("file created outside the disk")
	}
	// The backup folder itself must not be a link.
	m2 := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(m2, "Backup")); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenDiskRoot(m2, "Backup"); err == nil {
		t.Fatal("symlinked backup folder accepted")
	}
	// Nested target folders are created.
	r3, err := OpenDiskRoot(t.TempDir(), `Back-ups\Office`)
	if err != nil {
		t.Fatal(err)
	}
	r3.Close()
}
