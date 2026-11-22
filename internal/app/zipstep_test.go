package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/macsnoeren/backup-usb/internal/archive"
	"github.com/macsnoeren/backup-usb/internal/testutil"
)

func zipJob(t *testing.T) ZipJob {
	src := t.TempDir()
	testutil.WriteFile(t, src, "a.txt", strings.Repeat("a", 10000))
	testutil.WriteFile(t, src, "b/c.txt", "c")
	week := filepath.Join(t.TempDir(), "Backup", "2026", "47")
	if err := os.MkdirAll(week, 0o755); err != nil {
		t.Fatal(err)
	}
	return ZipJob{Src: src, Dest: filepath.Join(week, "Data.zip"), Opts: archive.Options{CompressionLevel: 6}, Verify: true}
}

func TestWriteVerifiedZip(t *testing.T) {
	job := zipJob(t)
	out, err := WriteVerifiedZip(context.Background(), job)
	if err != nil {
		t.Fatal(err)
	}
	sha, err := os.ReadFile(job.Dest + ".sha256")
	if err != nil {
		t.Fatal(err)
	}
	if string(sha) != out.SHA256+"  Data.zip\n" {
		t.Fatalf("sha256 file %q", sha)
	}
	sum, _, _ := HashFile(context.Background(), job.Dest)
	if sum != out.SHA256 {
		t.Fatal("sha256 does not match file")
	}
	got := testutil.Tree(t, filepath.Dir(job.Dest))
	if len(got) != 2 {
		t.Fatalf("week folder: %v", got)
	}
}

func TestSimulatedFailuresLeaveNothing(t *testing.T) {
	cases := map[string]func(part string) error{
		"write error": func(string) error { return errors.New("disk pulled out") },
		"corrupted header": func(part string) error {
			data, _ := os.ReadFile(part)
			data[31] ^= 0xff // first entry name
			return os.WriteFile(part, data, 0o600)
		},
		"corrupted central directory": func(part string) error {
			data, _ := os.ReadFile(part)
			data[len(data)-30] ^= 0x01
			return os.WriteFile(part, data, 0o600)
		},
		"corrupted data": func(part string) error {
			data, _ := os.ReadFile(part)
			data[len(data)/3] ^= 0xff
			return os.WriteFile(part, data, 0o600)
		},
	}
	for name, hook := range cases {
		job := zipJob(t)
		// An older good zip of the same week must survive.
		testutil.WriteFile(t, filepath.Dir(job.Dest), "Data.zip", "old good zip")
		testHookAfterWrite = hook
		_, err := WriteVerifiedZip(context.Background(), job)
		testHookAfterWrite = nil
		if err == nil {
			t.Fatalf("%s: no error", name)
		}
		want := KindIO
		if strings.HasPrefix(name, "corrupted") {
			want = KindVerify
		}
		if KindOf(err) != want {
			t.Errorf("%s: kind %v, want %v", name, KindOf(err), want)
		}
		got := testutil.Tree(t, filepath.Dir(job.Dest))
		if len(got) != 1 || got["Data.zip"] != "old good zip" {
			t.Errorf("%s: left behind %v", name, got)
		}
	}
}

func TestCleanupParts(t *testing.T) {
	dir := t.TempDir()
	for _, f := range []string{"2026/47/Data.zip.part", "2026/47/Data.zip.age.part", "2026/47/Data.zip.sha256.part", "2026/47/Data.zip", "2026/47/notes.part", "keep.txt", "2026/46/report.txt.part"} {
		testutil.WriteFile(t, dir, f, "x")
	}
	removed, err := CleanupParts(dir)
	if err != nil || len(removed) != 4 {
		t.Fatalf("removed %v, %v", removed, err)
	}
	got := testutil.Tree(t, dir)
	for _, keep := range []string{"2026/47/Data.zip", "2026/47/notes.part", "keep.txt"} {
		if _, ok := got[keep]; !ok {
			t.Errorf("%s removed", keep)
		}
	}
	if _, err := CleanupParts(filepath.Join(dir, "missing")); err != nil {
		t.Fatal(err)
	}
}

func TestCleanupPartsDoesNotFollowSymlinks(t *testing.T) {
	outside := t.TempDir()
	testutil.WriteFile(t, outside, "victim.zip.part", "precious")
	dir := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(dir, "link")); err != nil {
		t.Skip("symlinks not available:", err)
	}
	if _, err := CleanupParts(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(outside, "victim.zip.part")); err != nil {
		t.Fatal("file outside the backup folder was removed")
	}
}
