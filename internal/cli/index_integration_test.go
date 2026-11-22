//go:build integration

package cli

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"testing"

	"filippo.io/age"

	"github.com/macsnoeren/backup-usb/internal/app"
)

// TestIndexMatchesZip checks every index entry against the zip, for an
// unencrypted and an encrypted backup.
func TestIndexMatchesZip(t *testing.T) {
	for _, encrypted := range []bool{false, true} {
		w := newWorld(t)
		w.enrollUnsigned(map[string]string{"BACKUP1": "odd"})
		var ids []age.Identity
		if encrypted {
			bg := w.keysInit()
			data, _ := os.ReadFile(bg)
			ids, _, _ = app.ParseIdentityFile(data)
		}
		w.mustRun(0, "run")
		root, err := os.OpenRoot(w.diskFile("BACKUP1", "Backup"))
		if err != nil {
			t.Fatal(err)
		}
		for _, tag := range []string{"Data", "Books"} {
			if encrypted && exists(w.diskFile("BACKUP1", "Backup/2026/47/"+tag+".index.gz")) {
				t.Fatal("unencrypted index next to an encrypted backup")
			}
			ix, err := app.ReadIndex(root, "2026/47", tag, ids)
			if err != nil {
				t.Fatalf("%s: %v", tag, err)
			}
			var zr *zip.Reader
			name := filepath.Join(root.Name(), "2026", "47", tag+".zip")
			if encrypted {
				f, _ := os.Open(name + ".age")
				st, _ := f.Stat()
				ra, size, err := age.DecryptReaderAt(f, st.Size(), ids...)
				if err != nil {
					t.Fatal(err)
				}
				zr, err = zip.NewReader(ra, size)
				if err != nil {
					t.Fatal(err)
				}
				defer f.Close()
			} else {
				rc, err := zip.OpenReader(name)
				if err != nil {
					t.Fatal(err)
				}
				defer rc.Close()
				zr = &rc.Reader
			}
			byName := map[string]*zip.File{}
			for _, f := range zr.File {
				byName[f.Name] = f
			}
			if len(ix.Entries)+1 != len(zr.File) {
				t.Fatalf("%s: index %d entries, zip %d", tag, len(ix.Entries), len(zr.File))
			}
			for _, e := range ix.Entries {
				f := byName[e.Path]
				off, _ := f.DataOffset()
				if f == nil || off != e.DataOffset || f.CRC32 != e.CRC32 || f.UncompressedSize64 != e.Size || f.CompressedSize64 != e.CompressedSize || !f.Modified.Equal(e.Modified.Truncate(1e9)) && !e.Dir {
					t.Fatalf("%s: %s differs", tag, e.Path)
				}
			}
		}
		root.Close()
		_ = context.Background()
	}
}
