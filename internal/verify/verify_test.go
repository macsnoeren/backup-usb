package verify

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/macsnoeren/backup-usb/internal/archive"
	"github.com/macsnoeren/backup-usb/internal/testutil"
)

func writeZip(t *testing.T) (string, *archive.Result) {
	src := t.TempDir()
	testutil.WriteFile(t, src, "a.txt", strings.Repeat("alpha ", 5000))
	testutil.WriteFile(t, src, "dir/b.jpg", strings.Repeat("b", 3000))
	testutil.WriteFile(t, src, "dir/empty", "")
	dest := filepath.Join(t.TempDir(), "Data.zip")
	res, err := writeFileTo(t, src, dest, archive.Options{CompressionLevel: 6, StoreExtensions: []string{"jpg"}})
	if err != nil {
		t.Fatal(err)
	}
	return dest + ".part", res
}

func TestGoodZip(t *testing.T) {
	p, res := writeZip(t)
	rep, err := File(context.Background(), p, Options{ExpectedFiles: res.Files})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Files != 3 || rep.Bytes != res.Bytes {
		t.Fatalf("report %+v", rep)
	}
	if _, err := File(context.Background(), p, Options{ExpectedFiles: 4}); !errors.Is(err, ErrFailed) {
		t.Fatalf("wrong count accepted: %v", err)
	}
}

func TestChangedByteIsFound(t *testing.T) {
	p, res := writeZip(t)
	for _, e := range res.Entries {
		if e.Dir || e.CompressedSize == 0 {
			continue
		}
		data, _ := os.ReadFile(p)
		data[e.DataOffset+int64(e.CompressedSize)/2] ^= 0x01
		bad := filepath.Join(t.TempDir(), "bad.zip")
		if err := os.WriteFile(bad, data, 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := File(context.Background(), bad, Options{ExpectedFiles: -1}); !errors.Is(err, ErrFailed) {
			t.Errorf("changed byte in %s not found: %v", e.Path, err)
		}
	}
}

// craft builds a zip with the given files and manifest.
func craft(t *testing.T, files map[string]string, manifest string) []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, _ := zw.Create(name)
		w.Write([]byte(content))
	}
	if manifest != "-" {
		w, _ := zw.Create(archive.ManifestName)
		w.Write([]byte(manifest))
	}
	zw.Close()
	return buf.Bytes()
}

func check(data []byte) error {
	_, err := Zip(context.Background(), bytes.NewReader(data), int64(len(data)), Options{ExpectedFiles: -1})
	return err
}

func TestMissingEntryIsFound(t *testing.T) {
	sum := "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824" // sha256("hello")
	good := craft(t, map[string]string{"a": "hello"}, archive.ManifestLine(sum, "a"))
	if err := check(good); err != nil {
		t.Fatal(err)
	}
	cases := map[string][]byte{
		"entry missing":   craft(t, map[string]string{"a": "hello"}, archive.ManifestLine(sum, "a")+archive.ManifestLine(sum, "b")),
		"not in manifest": craft(t, map[string]string{"a": "hello", "b": "hello"}, archive.ManifestLine(sum, "a")),
		"hash mismatch":   craft(t, map[string]string{"a": "hellO"}, archive.ManifestLine(sum, "a")),
		"no manifest":     craft(t, map[string]string{"a": "hello"}, "-"),
		"bad manifest":    craft(t, map[string]string{"a": "hello"}, "nonsense\n"),
		"not a zip":       []byte("this is not a zip file at all"),
	}
	for name, data := range cases {
		if err := check(data); !errors.Is(err, ErrFailed) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func FuzzZip(f *testing.F) {
	sum := "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("a")
	w.Write([]byte("hello"))
	w, _ = zw.Create(archive.ManifestName)
	w.Write([]byte(archive.ManifestLine(sum, "a")))
	zw.Close()
	f.Add(buf.Bytes())
	f.Fuzz(func(t *testing.T, data []byte) {
		_ = check(data)
	})
}

// TestEveryByteIsCovered flips every byte of the local headers, data and
// data descriptors of a small zip in turn and expects the check (with the
// writer's entries) to notice each change. The central directory is covered
// by the stream hash in app.WriteVerifiedZip.
func TestEveryByteIsCovered(t *testing.T) {
	p, res := writeZip(t)
	data, _ := os.ReadFile(p)
	entries := res.AllEntries()
	if _, err := Zip(context.Background(), bytes.NewReader(data), int64(len(data)), Options{ExpectedFiles: res.Files, Entries: entries}); err != nil {
		t.Fatal(err)
	}
	missed := 0
	me := res.ManifestEntry
	end := me.DataOffset + int64(me.CompressedSize) + 16
	inDeflate := func(i int64) bool {
		for _, e := range entries {
			if e.Method == 8 && i >= e.DataOffset && i < e.DataOffset+int64(e.CompressedSize) {
				return true
			}
		}
		return false
	}
	for i := range data[:end] {
		if inDeflate(int64(i)) {
			// Some deflate bits (padding after the last block) do not
			// change the output; the stream hash covers them.
			continue
		}
		bad := append([]byte(nil), data...)
		bad[i] ^= 0x01
		_, err := Zip(context.Background(), bytes.NewReader(bad), int64(len(bad)), Options{ExpectedFiles: res.Files, Entries: entries})
		if err == nil {
			missed++
			t.Logf("change at offset %d not detected", i)
		}
	}
	if missed > 0 {
		t.Errorf("%d byte changes not detected", missed)
	}
	// Changes inside deflate data are either detected or harmless.
	for _, e := range entries {
		if e.Method != 8 {
			continue
		}
		for i := e.DataOffset; i < e.DataOffset+int64(e.CompressedSize); i++ {
			bad := append([]byte(nil), data...)
			bad[i] ^= 0x01
			zr, err := Zip(context.Background(), bytes.NewReader(bad), int64(len(bad)), Options{ExpectedFiles: res.Files, Entries: entries})
			_ = zr
			if err == nil {
				t.Logf("%s: flip at %d is harmless (output unchanged, checked by SHA-256)", e.Path, i)
			}
		}
	}
}

func writeFileTo(t *testing.T, src, dest string, opts archive.Options) (*archive.Result, error) {
	t.Helper()
	root, err := os.OpenRoot(filepath.Dir(dest))
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	return archive.WriteFile(context.Background(), src, root, filepath.Base(dest), opts)
}
