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

package main

import (
	"bytes"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	_ "github.com/macsnoeren/backup-usb/internal/testutil" // -update flag
)

// The committed files must match the drawing ("make icon"). The PNG inside
// depends on the Go version, so only the bitmaps are compared.
func TestIconFilesUpToDate(t *testing.T) {
	if runtime.GOARCH != "amd64" {
		t.Skip("rounding differs per processor")
	}
	all, err := files()
	if err != nil {
		t.Fatal(err)
	}
	imgs, err := images()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 6 {
		t.Fatalf("%d files", len(all))
	}
	for name := range all {
		got, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(name)))
		if err != nil {
			t.Error(err)
			continue
		}
		if filepath.Ext(name) == ".png" {
			img, err := png.Decode(bytes.NewReader(got))
			if err != nil || img.Bounds().Dx() != 256 {
				t.Errorf("%s: %v", name, err)
			}
			continue
		}
		for _, im := range imgs {
			if im.size < 256 && !bytes.Contains(got, im.data) {
				t.Errorf("%s: the %d pixel image differs; run make icon", name, im.size)
			}
		}
	}
}
