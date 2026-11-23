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

//go:build !windows

package archive

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/macsnoeren/backup-usb/internal/testutil"
)

func TestOwnerAndPermissionsInZip(t *testing.T) {
	src := t.TempDir()
	p := testutil.WriteFile(t, src, "private.txt", "x")
	os.Chmod(p, 0o640)
	os.Symlink("private.txt", filepath.Join(src, "link"))
	var buf bytes.Buffer
	res, err := Write(context.Background(), src, &buf, defaultOpts())
	if err != nil {
		t.Fatal(err)
	}
	zr, _ := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	uid, gid := uint32(os.Getuid()), uint32(os.Getgid())
	for _, f := range zr.File {
		if f.Name == ManifestName {
			continue
		}
		i := bytes.Index(f.Extra, []byte{0x75, 0x78, 11, 0, 1, 4})
		if i < 0 || binary.LittleEndian.Uint32(f.Extra[i+6:]) != uid || binary.LittleEndian.Uint32(f.Extra[i+11:]) != gid {
			t.Fatalf("%s: no ux owner field: %x", f.Name, f.Extra)
		}
		if f.Name == "private.txt" && f.Mode().Perm() != 0o640 {
			t.Fatalf("mode %v", f.Mode())
		}
		if f.Name == "link" && f.Mode()&os.ModeSymlink == 0 {
			t.Fatal("link not stored as link")
		}
	}
	for _, e := range res.Entries {
		if e.UID == nil || *e.UID != uid {
			t.Fatalf("%s: owner not in index", e.Path)
		}
	}
}
