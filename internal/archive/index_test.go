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
	"bytes"
	"compress/gzip"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/macsnoeren/backup-usb/internal/testutil"
)

func TestIndexRoundTrip(t *testing.T) {
	src := t.TempDir()
	testutil.WriteFile(t, src, "a/b.txt", "b")
	var zipBuf bytes.Buffer
	res, err := Write(context.Background(), src, &zipBuf, defaultOpts())
	if err != nil {
		t.Fatal(err)
	}
	ix := NewIndex("Data", "2026-W47", "Data.zip", time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC), res)
	var buf bytes.Buffer
	if err := ix.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	back, err := DecodeIndex(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if len(back.Entries) != 2 || back.Entries[1].Path != "a/b.txt" || back.Entries[1].SHA256 != res.Entries[1].SHA256 || back.ZipSize != res.ZipSize {
		t.Fatalf("index %+v", back)
	}
}

func TestIndexRejectsBadPaths(t *testing.T) {
	for _, bad := range []string{`{"version":1,"entries":[{"path":"../x"}]}`, `{"version":1,"entries":[{"path":"/etc/passwd"}]}`, `{"version":2}`, `{"version":1,"zipSize":10,"entries":[{"path":"a","headerOffset":5,"dataOffset":2}]}`} {
		var buf bytes.Buffer
		zw := gzip.NewWriter(&buf)
		zw.Write([]byte(bad))
		zw.Close()
		if _, err := DecodeIndex(&buf); err == nil {
			t.Errorf("accepted %s", bad)
		}
	}
	if _, err := DecodeIndex(strings.NewReader("not gzip")); err == nil {
		t.Fatal("accepted garbage")
	}
}

func FuzzDecodeIndex(f *testing.F) {
	var buf bytes.Buffer
	(&Index{Version: 1, Entries: []Entry{{Path: "a"}}}).Encode(&buf)
	f.Add(buf.Bytes())
	f.Fuzz(func(t *testing.T, data []byte) { _, _ = DecodeIndex(bytes.NewReader(data)) })
}
