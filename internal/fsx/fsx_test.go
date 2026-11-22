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
	bad := []string{"", "/etc/passwd", "../x", "a/../../x", "a/./b", "a//b", `a\b`, `..\x`, "C:/x", "c:x", "//server/share", "a\x00b", "a\nb", "CON", "dir/nul.txt", "LPT1", "..", "."}
	for _, b := range bad {
		if _, err := SafeJoin(base, b); err == nil {
			t.Errorf("%q accepted", b)
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
