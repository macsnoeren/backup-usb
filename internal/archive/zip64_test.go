// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Maurice Snoeren
//
// This file is part of USB Backup.
//
// USB Backup is free software: you can redistribute it and/or modify it under
// the terms of the GNU General Public License as published by the Free Software
// Foundation, either version 3 of the License, or (at your option) any later
// version.
//
// USB Backup is distributed in the hope that it will be useful, but WITHOUT ANY
// WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A
// PARTICULAR PURPOSE. See the GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License along with
// USB Backup. If not, see <https://www.gnu.org/licenses/>.

package archive

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestStoreExtensions(t *testing.T) {
	src := t.TempDir()
	for _, name := range []string{"a.jpg", "b.PNG", "c.mp4", "d.docx", "e.txt", "f", "g.jpg.txt"} {
		if err := os.WriteFile(filepath.Join(src, name), make([]byte, 4096), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	opts := defaultOpts()
	opts.StoreExtensions = []string{"jpg", ".png", "mp4", "docx"}
	dest := filepath.Join(t.TempDir(), "x.zip")
	if _, err := WriteFileTo(t, src, dest, opts); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.OpenReader(dest + ".part")
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	want := map[string]uint16{"a.jpg": zip.Store, "b.PNG": zip.Store, "c.mp4": zip.Store, "d.docx": zip.Store, "e.txt": zip.Deflate, "f": zip.Deflate, "g.jpg.txt": zip.Deflate}
	for _, f := range zr.File {
		if m, ok := want[f.Name]; ok && f.Method != m {
			t.Errorf("%s: method %d, want %d", f.Name, f.Method, m)
		}
	}
}

func TestManyFilesUseZip64(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	src := t.TempDir()
	const dirs, perDir = 70, 1000
	for d := 0; d < dirs; d++ {
		dir := filepath.Join(src, fmt.Sprintf("d%02d", d))
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < perDir; i++ {
			if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("f%04d.txt", i)), []byte{byte(i)}, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	dest := filepath.Join(t.TempDir(), "many.zip")
	res, err := WriteFileTo(t, src, dest, defaultOpts())
	if err != nil {
		t.Fatal(err)
	}
	if res.Files != dirs*perDir {
		t.Fatalf("files %d", res.Files)
	}
	zr, err := zip.OpenReader(dest + ".part")
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	if got := len(zr.File); got != dirs*perDir+dirs+1 {
		t.Fatalf("zip has %d entries", got)
	}
	if !hasZip64EOCD(t, dest+".part") {
		t.Fatal("no zip64 end of central directory record")
	}
}

// hasZip64EOCD looks for the zip64 end of central directory locator.
func hasZip64EOCD(t *testing.T, path string) bool {
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	st, _ := f.Stat()
	buf := make([]byte, 22+20)
	if _, err := f.ReadAt(buf, st.Size()-int64(len(buf))); err != nil {
		t.Fatal(err)
	}
	return buf[0] == 0x50 && buf[1] == 0x4b && buf[2] == 0x06 && buf[3] == 0x07
}
