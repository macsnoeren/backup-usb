package archive

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"

	"github.com/macsnoeren/backup-usb/internal/testutil"
)

func names(res *Result) []string {
	var out []string
	for _, e := range res.Entries {
		out = append(out, e.Path)
	}
	sort.Strings(out)
	return out
}

func TestExclude(t *testing.T) {
	src := t.TempDir()
	for _, f := range []string{"keep.txt", "x.tmp", "~$doc.docx", "Thumbs.db", "sub/desktop.ini", "sub/keep.doc", "cache/big.bin", "sub/cache/keep.bin", "logs/a.log"} {
		testutil.WriteFile(t, src, f, "x")
	}
	opts := defaultOpts()
	// Global defaults plus per-source patterns, as run combines them.
	opts.Exclude = []string{"*.tmp", "~$*", "Thumbs.db", "desktop.ini", "cache", "logs/*.log"}
	var buf bytes.Buffer
	res, err := Write(context.Background(), src, &buf, opts)
	if err != nil {
		t.Fatal(err)
	}
	got := names(res)
	want := []string{"keep.txt", "logs/", "sub/", "sub/keep.doc"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
	files, bytes, err := Scan(src, opts)
	if err != nil || files != 2 || bytes != 2 {
		t.Fatalf("scan: %d files %d bytes %v", files, bytes, err)
	}
}

func lockedSource(t *testing.T) string {
	if runtime.GOOS == "windows" {
		t.Skip("chmod does not make files unreadable on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root can read everything")
	}
	src := t.TempDir()
	testutil.WriteFile(t, src, "ok.txt", "fine")
	p := testutil.WriteFile(t, src, "outlook.pst", "locked")
	if err := os.Chmod(p, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(p, 0o644) })
	return src
}

func TestLockedSkip(t *testing.T) {
	src := lockedSource(t)
	var buf bytes.Buffer
	res, err := Write(context.Background(), src, &buf, defaultOpts())
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Skipped) != 1 || res.Skipped[0].Path != "outlook.pst" || res.Files != 1 {
		t.Fatalf("skipped %+v, files %d", res.Skipped, res.Files)
	}
}

func TestLockedFail(t *testing.T) {
	src := lockedSource(t)
	opts := defaultOpts()
	opts.LockedFiles = LockedFail
	dest := filepath.Join(t.TempDir(), "x.zip")
	_, err := WriteFile(context.Background(), src, dest, opts)
	if !errors.Is(err, ErrLocked) {
		t.Fatalf("err = %v", err)
	}
	if _, err := os.Stat(dest + ".part"); !os.IsNotExist(err) {
		t.Fatal("part file left behind")
	}
}
