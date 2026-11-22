package archive

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/macsnoeren/backup-usb/internal/testutil"
)

func defaultOpts() Options {
	return Options{CompressionLevel: 6, StoreExtensions: []string{"jpg", "zip"}, LockedFiles: LockedSkip}
}

// makeSource builds a source tree with nested folders, an empty folder,
// Unicode names and empty files.
func makeSource(t *testing.T) string {
	src := t.TempDir()
	testutil.WriteFile(t, src, "readme.txt", strings.Repeat("hello world\n", 1000))
	testutil.WriteFile(t, src, "a/b/c/deep.txt", "deep")
	testutil.WriteFile(t, src, "a/empty.bin", "")
	testutil.WriteFile(t, src, "Ünïcødé/naïve €.txt", "unicode content")
	testutil.WriteFile(t, src, "photos/img.JPG", string(bytes.Repeat([]byte{0xff, 0xd8, 1, 2, 3}, 5000)))
	if err := os.MkdirAll(filepath.Join(src, "empty-dir", "nested-empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	old := time.Date(2020, 5, 17, 13, 45, 30, 0, time.UTC)
	if err := os.Chtimes(filepath.Join(src, "readme.txt"), old, old); err != nil {
		t.Fatal(err)
	}
	return src
}

// extract unpacks a zip into a new folder with archive/zip and returns it.
func extract(t *testing.T, zipPath string) string {
	t.Helper()
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	out := t.TempDir()
	for _, f := range zr.File {
		p := filepath.Join(out, filepath.FromSlash(f.Name))
		if strings.HasSuffix(f.Name, "/") {
			if err := os.MkdirAll(p, 0o755); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatalf("%s: %v", f.Name, err)
		}
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, f.Modified, f.Modified); err != nil {
			t.Fatal(err)
		}
	}
	return out
}

func TestRoundTrip(t *testing.T) {
	src := makeSource(t)
	dest := filepath.Join(t.TempDir(), "Data.zip")
	res, err := WriteFile(context.Background(), src, dest, defaultOpts())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dest + ".part"); err != nil {
		t.Fatal("expected the zip to be written as .part:", err)
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatal("final name must not exist before verification")
	}
	out := extract(t, dest+".part")
	want := testutil.Tree(t, src)
	got := testutil.Tree(t, out)
	if got["_backup/manifest.sha256"] == "" {
		t.Fatal("manifest missing")
	}
	delete(got, "_backup/manifest.sha256")
	delete(got, "_backup")
	if len(got) != len(want) {
		t.Fatalf("entries: got %d want %d\n%v\n%v", len(got), len(want), got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s differs", k)
		}
	}
	if res.Files != 5 || res.Dirs != 7 {
		t.Errorf("files %d dirs %d", res.Files, res.Dirs)
	}
	st, _ := os.Stat(filepath.Join(out, "readme.txt"))
	if !st.ModTime().Equal(time.Date(2020, 5, 17, 13, 45, 30, 0, time.UTC)) {
		t.Errorf("modification time not kept: %v", st.ModTime())
	}
}

func TestZipDetails(t *testing.T) {
	src := makeSource(t)
	dest := filepath.Join(t.TempDir(), "Data.zip")
	res, err := WriteFile(context.Background(), src, dest, defaultOpts())
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.OpenReader(dest + ".part")
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	byName := map[string]*zip.File{}
	for _, f := range zr.File {
		byName[f.Name] = f
		if strings.Contains(f.Name, `\`) {
			t.Errorf("backslash in %q", f.Name)
		}
	}
	if zr.File[len(zr.File)-1].Name != ManifestName {
		t.Fatal("manifest is not the last entry")
	}
	u := byName["Ünïcødé/naïve €.txt"]
	if u == nil || u.Flags&0x800 == 0 {
		t.Fatal("UTF-8 flag missing on unicode name")
	}
	if f := byName["photos/img.JPG"]; f == nil || f.Method != zip.Store {
		t.Fatal("jpg should be stored")
	}
	if f := byName["readme.txt"]; f == nil || f.Method != zip.Deflate || f.CompressedSize64 >= f.UncompressedSize64 {
		t.Fatal("txt should be deflated")
	}
	if byName["empty-dir/nested-empty/"] == nil {
		t.Fatal("empty folder missing")
	}
	// Offsets in the result match the zip.
	for _, e := range res.Entries {
		f := byName[e.Path]
		if f == nil {
			t.Fatalf("%s not in zip", e.Path)
		}
		off, err := f.DataOffset()
		if err != nil || off != e.DataOffset {
			t.Errorf("%s: data offset %d, zip says %d (%v)", e.Path, e.DataOffset, off, err)
		}
		if f.CRC32 != e.CRC32 || f.CompressedSize64 != e.CompressedSize || f.Method != e.Method {
			t.Errorf("%s: header mismatch", e.Path)
		}
	}
	if fi, _ := os.Stat(dest + ".part"); fi.Size() != res.ZipSize {
		t.Errorf("zip size %d, file %d", res.ZipSize, fi.Size())
	}
}

func TestErrorsRemovePart(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "Data.zip")
	if _, err := WriteFile(context.Background(), filepath.Join(t.TempDir(), "missing"), dest, defaultOpts()); err == nil {
		t.Fatal("missing source accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := WriteFile(ctx, makeSource(t), dest, defaultOpts()); err == nil {
		t.Fatal("cancelled write succeeded")
	}
	if _, err := os.Stat(dest + ".part"); !os.IsNotExist(err) {
		t.Fatal("part file left behind")
	}
}

func TestCompressionZeroStoresAll(t *testing.T) {
	src := makeSource(t)
	var buf bytes.Buffer
	opts := defaultOpts()
	opts.CompressionLevel = 0
	res, err := Write(context.Background(), src, &buf, opts)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range res.Entries {
		if !e.Dir && e.Method != zip.Store {
			t.Errorf("%s deflated with level 0", e.Path)
		}
	}
}

func TestSymlinkStoredAsLink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on Windows")
	}
	src := t.TempDir()
	testutil.WriteFile(t, src, "target.txt", "x")
	if err := os.Symlink("target.txt", filepath.Join(src, "link")); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	res, err := Write(context.Background(), src, &buf, defaultOpts())
	if err != nil {
		t.Fatal(err)
	}
	var link *Entry
	for i := range res.Entries {
		if res.Entries[i].Path == "link" {
			link = &res.Entries[i]
		}
	}
	if link == nil || link.Link != "target.txt" {
		t.Fatalf("link entry: %+v", link)
	}
	zr, _ := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	for _, f := range zr.File {
		if f.Name == "link" && f.Mode()&os.ModeSymlink == 0 {
			t.Fatal("zip entry is not a symlink")
		}
	}
}
