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
	_, err := WriteFileTo(t, src, dest, opts)
	if !errors.Is(err, ErrLocked) {
		t.Fatalf("err = %v", err)
	}
	if _, err := os.Stat(dest + ".part"); !os.IsNotExist(err) {
		t.Fatal("part file left behind")
	}
}

func TestUnreadableFolderIsReported(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs an unreadable folder")
	}
	src := t.TempDir()
	testutil.WriteFile(t, src, "secret/a.txt", "a")
	testutil.WriteFile(t, src, "ok.txt", "ok")
	if err := os.Chmod(filepath.Join(src, "secret"), 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(src, "secret"), 0o755) })
	var buf bytes.Buffer
	res, err := Write(context.Background(), src, &buf, defaultOpts())
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Skipped) != 1 || res.Skipped[0].Path != "secret/" {
		t.Fatalf("skipped %+v", res.Skipped)
	}
	opts := defaultOpts()
	opts.LockedFiles = LockedFail
	if _, err := Write(context.Background(), src, &buf, opts); !errors.Is(err, ErrLocked) {
		t.Fatalf("fail mode: %v", err)
	}
}

func TestUnrestorableNames(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("these names cannot be created on Windows")
	}
	src := t.TempDir()
	testutil.WriteFile(t, src, "2024-01-01 10:00.txt", "colon")
	testutil.WriteFile(t, src, "line\nbreak.txt", "newline")
	var buf bytes.Buffer
	res, err := Write(context.Background(), src, &buf, defaultOpts())
	if err != nil {
		t.Fatal(err)
	}
	if res.Files != 1 || len(res.Skipped) != 1 || res.Skipped[0].Path != "line\nbreak.txt" {
		t.Fatalf("files %d skipped %+v", res.Files, res.Skipped)
	}
	if len(res.Warnings) != 1 || res.Warnings[0].Reason != "name cannot be restored on Windows" {
		t.Fatalf("warnings %+v", res.Warnings)
	}
}

func TestFileSwappedForDevice(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no /dev/zero")
	}
	src := t.TempDir()
	p := testutil.WriteFile(t, src, "x.txt", "x")
	info, _ := os.Lstat(p)
	// Simulate a swap between listing and opening: the open handle is a
	// different file than the one that was listed.
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/dev/zero", p); err != nil {
		t.Fatal(err)
	}
	f, _ := os.Open(p)
	st, _ := f.Stat()
	f.Close()
	if st.Mode().IsRegular() || os.SameFile(st, info) {
		t.Fatal("test setup: swap not visible on the handle")
	}
}
