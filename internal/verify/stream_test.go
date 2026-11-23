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

package verify

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/macsnoeren/backup-usb/internal/archive"
	"github.com/macsnoeren/backup-usb/internal/testutil"
)

func streamSource(t *testing.T) string {
	src := t.TempDir()
	testutil.WriteFile(t, src, "a.txt", strings.Repeat("alpha ", 20000))
	testutil.WriteFile(t, src, "photo.jpg", strings.Repeat("\xff\xd8jpeg", 30000))
	testutil.WriteFile(t, src, "empty.jpg", "")
	testutil.WriteFile(t, src, "empty.txt", "")
	testutil.WriteFile(t, src, "dir/sub/b.txt", "b")
	os.MkdirAll(filepath.Join(src, "emptydir"), 0o755)
	if runtime.GOOS != "windows" {
		os.Symlink("a.txt", filepath.Join(src, "link"))
	}
	return src
}

// writeThrough writes a zip of src into a stream verifier and a buffer.
type through struct {
	res     *archive.Result
	entries []StreamEntry
	data    []byte
	ends    map[int64]int64
	err     error
}

func writeThrough(t *testing.T, src string, opts archive.Options) through {
	s := NewStream()
	var buf bytes.Buffer
	ends := map[int64]int64{}
	opts.StoredEnd = func(off, end int64) { ends[off] = end; s.StoredEnd(off, end) }
	res, err := archive.Write(context.Background(), src, &multi{&buf, s.Writer()}, opts)
	if err != nil {
		s.Abort(err)
		t.Fatal(err)
	}
	entries, verr := s.Finish()
	return through{res, entries, buf.Bytes(), ends, verr}
}

type multi struct {
	a, b interface{ Write([]byte) (int, error) }
}

func (m *multi) Write(p []byte) (int, error) {
	if n, err := m.a.Write(p); err != nil {
		return n, err
	}
	return m.b.Write(p)
}

func TestStreamGood(t *testing.T) {
	for _, level := range []int{0, 1, 9} {
		opts := archive.Options{CompressionLevel: level, StoreExtensions: []string{"jpg"}}
		w := writeThrough(t, streamSource(t), opts)
		res, entries, err, data := w.res, w.entries, w.err, w.data
		if err != nil {
			t.Fatalf("level %d: %v", level, err)
		}
		all := res.AllEntries()
		if len(entries) != len(all) {
			t.Fatalf("level %d: %d entries, want %d", level, len(entries), len(all))
		}
		for i, e := range all {
			g := entries[i]
			if g.Name != e.Path || g.CRC32 != e.CRC32 || g.Size != e.Size || g.CompressedSize != e.CompressedSize || g.DataOffset != e.DataOffset || (!e.Dir && g.SHA256 != e.SHA256) {
				t.Fatalf("level %d: entry %d differs:\n%+v\n%+v", level, i, g, e)
			}
		}
		// The same bytes also pass the normal check.
		if _, err := Zip(context.Background(), bytes.NewReader(data), int64(len(data)), Options{ExpectedFiles: res.Files}); err != nil {
			t.Fatal(err)
		}
	}
}

// TestStreamCatchesEveryByte flips each byte of a small zip and feeds it to
// a fresh verifier with the same announcements.
func TestStreamCatchesEveryByte(t *testing.T) {
	src := t.TempDir()
	testutil.WriteFile(t, src, "a.txt", "hello hello hello hello")
	testutil.WriteFile(t, src, "b.jpg", "stored bytes")
	testutil.WriteFile(t, src, "d/c.txt", "")
	w := writeThrough(t, src, archive.Options{CompressionLevel: 6, StoreExtensions: []string{"jpg"}})
	res, err, data, ends := w.res, w.err, w.data, w.ends
	if err != nil {
		t.Fatal(err)
	}
	inDeflate := func(i int64) bool {
		for _, e := range res.AllEntries() {
			if e.Method == 8 && i >= e.DataOffset && i < e.DataOffset+int64(e.CompressedSize) {
				return true
			}
		}
		return false
	}
	missed := 0
	for i := range data {
		bad := append([]byte(nil), data...)
		bad[i] ^= 0x01
		s := NewStream()
		for k, v := range ends {
			s.StoredEnd(k, v)
		}
		go func() { s.Writer().Write(bad); s.pw.Close() }()
		<-s.done
		if s.err == nil && !inDeflate(int64(i)) {
			missed++
			t.Logf("flip at %d not detected", i)
		}
	}
	// Some fields of the central directory are not compared (version made
	// by, attributes, dates); those are covered by the SHA-256 over the
	// encrypted bytes. Everything else must be caught.
	// Per central entry: version made by, internal and external attributes.
	if missed > 8*len(res.AllEntries()) {
		t.Fatalf("%d flips not detected", missed)
	}
}

func TestStreamManyFiles(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	src := t.TempDir()
	for d := 0; d < 70; d++ {
		for i := 0; i < 1000; i++ {
			testutil.WriteFile(t, src, fmt.Sprintf("d%02d/f%04d.jpg", d, i), "x")
		}
	}
	w := writeThrough(t, src, archive.Options{CompressionLevel: 6, StoreExtensions: []string{"jpg"}})
	entries, err := w.entries, w.err
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 70*1000+70+1 {
		t.Fatalf("%d entries", len(entries))
	}
}

func TestStreamGarbage(t *testing.T) {
	for _, data := range [][]byte{nil, []byte("PK\x03\x04"), []byte("not a zip"), bytes.Repeat([]byte{0}, 100)} {
		s := NewStream()
		go func() { s.Writer().Write(data); s.pw.Close() }()
		if _, err := s.Finish(); !errors.Is(err, ErrFailed) {
			t.Errorf("%q accepted: %v", data, err)
		}
	}
}

func FuzzStream(f *testing.F) {
	src := f.TempDir()
	os.WriteFile(filepath.Join(src, "a.txt"), []byte("hello"), 0o644)
	var buf bytes.Buffer
	archive.Write(context.Background(), src, &buf, archive.Options{CompressionLevel: 6})
	f.Add(buf.Bytes())
	f.Fuzz(func(t *testing.T, data []byte) {
		s := NewStream()
		go func() { s.Writer().Write(data); s.pw.Close() }()
		_, _ = s.Finish()
	})
}
