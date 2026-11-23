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
	"bufio"
	"bytes"
	"compress/flate"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"hash/crc32"
	"io"
	"sync"

	"github.com/macsnoeren/backup-usb/internal/archive"
)

// StreamEntry is one entry as seen by the stream verifier.
type StreamEntry struct {
	Name           string
	Method         uint16
	CRC32          uint32
	CompressedSize uint64
	Size           uint64
	SHA256         string
	HeaderOffset   int64
	DataOffset     int64

	version, flags, mtime, mdate uint16
	extra                        []byte
}

// Stream verifies a zip while it is being written, without storing it: the
// writer copies every zip byte into the verifier (a pipe), and announces
// where stored entries end through StoredEnd. Every entry is unpacked, its
// CRC-32 and SHA-256 are computed from the bytes, the data descriptors,
// central directory and end records are parsed and compared, and the
// manifest inside the zip is checked against the computed hashes.
type Stream struct {
	pr   *io.PipeReader
	pw   *io.PipeWriter
	mu   sync.Mutex
	ends map[int64]int64
	cond *sync.Cond
	done chan struct{}

	entries []StreamEntry
	err     error
}

// NewStream starts a stream verifier.
func NewStream() *Stream {
	s := &Stream{ends: map[int64]int64{}, done: make(chan struct{})}
	s.cond = sync.NewCond(&s.mu)
	s.pr, s.pw = io.Pipe()
	go s.run()
	return s
}

// Writer is where the zip bytes go.
func (s *Stream) Writer() io.Writer { return s.pw }

// StoredEnd records where a stored entry's data ends (archive.Options).
func (s *Stream) StoredEnd(dataOffset, end int64) {
	s.mu.Lock()
	s.ends[dataOffset] = end
	s.mu.Unlock()
}

func (s *Stream) storedEnd(dataOffset int64) (int64, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.ends[dataOffset]
	return e, ok
}

// Abort stops the verifier after a write error.
func (s *Stream) Abort(err error) {
	_ = s.pw.CloseWithError(err)
	<-s.done
}

// Finish closes the stream and returns the entries the verifier found, or
// the first problem.
func (s *Stream) Finish() ([]StreamEntry, error) {
	_ = s.pw.Close()
	<-s.done
	return s.entries, s.err
}

func (s *Stream) run() {
	defer close(s.done)
	r := &posReader{br: bufio.NewReaderSize(s.pr, 64<<10)}
	s.entries, s.err = s.parse(r)
	if s.err != nil {
		_ = s.pr.CloseWithError(s.err)
	} else {
		_ = s.pr.Close()
	}
}

// posReader tracks the absolute position of consumed bytes.
type posReader struct {
	br  *bufio.Reader
	pos int64
}

func (p *posReader) Read(b []byte) (int, error) {
	n, err := p.br.Read(b)
	p.pos += int64(n)
	return n, err
}

func (p *posReader) ReadByte() (byte, error) {
	c, err := p.br.ReadByte()
	if err == nil {
		p.pos++
	}
	return c, err
}

func (p *posReader) full(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := io.ReadFull(p, b)
	return b, err
}

const (
	sigLocal       = 0x04034b50
	sigDescriptor  = 0x08074b50
	sigCentral     = 0x02014b50
	sigZip64End    = 0x06064b50
	sigZip64Locate = 0x07064b50
	sigEnd         = 0x06054b50
	maxEntries     = MaxEntries
)

func (s *Stream) parse(r *posReader) ([]StreamEntry, error) {
	var entries []StreamEntry
	var manifest []byte
	le := binary.LittleEndian
	for {
		start := r.pos
		sigb, err := r.full(4)
		if err != nil {
			return nil, fail("zip ends early")
		}
		sig := le.Uint32(sigb)
		if sig == sigCentral {
			if err := s.parseCentral(r, entries, start); err != nil {
				return nil, err
			}
			break
		}
		if sig != sigLocal {
			return nil, fail("unexpected record at offset %d", start)
		}
		if len(entries) >= maxEntries {
			return nil, fail("too many entries")
		}
		h, err := r.full(26)
		if err != nil {
			return nil, fail("local header truncated")
		}
		flags, method := le.Uint16(h[2:]), le.Uint16(h[4:])
		nameLen, extraLen := int(le.Uint16(h[22:])), int(le.Uint16(h[24:]))
		name, err := r.full(nameLen)
		if err != nil {
			return nil, fail("local header truncated")
		}
		extra, err := r.full(extraLen)
		if err != nil {
			return nil, fail("local header truncated")
		}
		// With a data descriptor, and for folders, CRC and sizes in the
		// local header are 0.
		if le.Uint32(h[10:]) != 0 || le.Uint32(h[14:]) != 0 || le.Uint32(h[18:]) != 0 {
			return nil, fail("local header of %q is damaged", name)
		}
		e := StreamEntry{Name: string(name), Method: method, HeaderOffset: start, DataOffset: r.pos,
			version: le.Uint16(h[0:]), flags: flags, mtime: le.Uint16(h[6:]), mdate: le.Uint16(h[8:]), extra: extra}
		isDir := len(name) > 0 && name[len(name)-1] == '/'
		crc := crc32.NewIEEE()
		sh := sha256.New()
		var keep *bytes.Buffer
		if e.Name == archive.ManifestName {
			keep = &bytes.Buffer{}
		}
		switch {
		case isDir:
			if method != 0 || flags&0x8 != 0 {
				return nil, fail("folder entry %q has data", e.Name)
			}
			entries = append(entries, e)
			continue
		case method == 8:
			fr := flate.NewReader(r)
			n, err := copyLimited(fr, crc, sh, keep)
			_ = fr.Close()
			if err != nil {
				return nil, fail("%q: %v", e.Name, err)
			}
			e.Size = n
		case method == 0:
			n, err := s.readStored(r, e.DataOffset, crc, sh, keep)
			if err != nil {
				return nil, fail("%q: %v", e.Name, err)
			}
			e.Size = n
		default:
			return nil, fail("%q: unsupported method %d", e.Name, method)
		}
		e.CompressedSize = uint64(r.pos - e.DataOffset) //nolint:gosec // positions only grow
		e.CRC32 = crc.Sum32()
		e.SHA256 = hex.EncodeToString(sh.Sum(nil))
		if flags&0x8 == 0 {
			return nil, fail("%q: expected a data descriptor", e.Name)
		}
		zip64 := e.CompressedSize >= 0xffffffff || e.Size >= 0xffffffff
		ddLen := 16
		if zip64 {
			ddLen = 24
		}
		dd, err := r.full(ddLen)
		if err != nil {
			return nil, fail("%q: data descriptor truncated", e.Name)
		}
		ok := le.Uint32(dd) == sigDescriptor && le.Uint32(dd[4:]) == e.CRC32
		if zip64 {
			ok = ok && le.Uint64(dd[8:]) == e.CompressedSize && le.Uint64(dd[16:]) == e.Size
		} else {
			ok = ok && uint64(le.Uint32(dd[8:])) == e.CompressedSize && uint64(le.Uint32(dd[12:])) == e.Size
		}
		if !ok {
			return nil, fail("%q: CRC-32 or size does not match the data descriptor", e.Name)
		}
		if keep != nil {
			manifest = keep.Bytes()
		}
		entries = append(entries, e)
	}
	// Trailing data after the end record is not allowed.
	if _, err := r.ReadByte(); !errors.Is(err, io.EOF) {
		return nil, fail("data after the end of the zip")
	}
	if manifest == nil {
		return nil, fail("manifest missing")
	}
	m, err := archive.ParseManifest(bytes.NewReader(manifest))
	if err != nil {
		return nil, fail("manifest: %v", err)
	}
	files := 0
	for _, e := range entries {
		if e.Name == archive.ManifestName || (len(e.Name) > 0 && e.Name[len(e.Name)-1] == '/') {
			continue
		}
		files++
		if m[e.Name] != e.SHA256 {
			return nil, fail("%q: SHA-256 does not match the manifest", e.Name)
		}
	}
	if files != len(m) {
		return nil, fail("manifest lists %d files, the zip has %d", len(m), files)
	}
	return entries, nil
}

// copyLimited copies a decompressed entry into the hashes, keeping up to
// MaxManifestSize bytes if keep is set.
func copyLimited(src io.Reader, crc, sh hash.Hash, keep *bytes.Buffer) (uint64, error) {
	buf := make([]byte, 64<<10)
	var n uint64
	for {
		m, err := src.Read(buf)
		if m > 0 {
			crc.Write(buf[:m])
			sh.Write(buf[:m])
			n += uint64(m)
			if keep != nil {
				if keep.Len()+m > archive.MaxManifestSize {
					return n, errors.New("manifest too large")
				}
				keep.Write(buf[:m])
			}
		}
		if errors.Is(err, io.EOF) {
			return n, nil
		}
		if err != nil {
			return n, err
		}
	}
}

// readStored reads a stored entry whose end is announced by the writer.
func (s *Stream) readStored(r *posReader, dataOffset int64, crc, sh hash.Hash, keep *bytes.Buffer) (uint64, error) {
	var n uint64
	consume := func(k int) error {
		b, err := r.full(k)
		if err != nil {
			return err
		}
		crc.Write(b)
		sh.Write(b)
		if keep != nil {
			keep.Write(b)
		}
		n += uint64(k) //nolint:gosec // k is positive
		return nil
	}
	for {
		if end, ok := s.storedEnd(dataOffset); ok {
			if end < r.pos {
				return n, errors.New("stored data overran its end")
			}
			left := end - r.pos
			for left > 0 {
				k := int(min(left, 64<<10))
				if err := consume(k); err != nil {
					return n, err
				}
				left -= int64(k)
			}
			return n, nil
		}
		if r.br.Buffered() == 0 {
			if _, err := r.br.Peek(1); err != nil {
				return n, fmt.Errorf("stored data truncated: %w", err)
			}
			continue // check the announcement again after waiting
		}
		// Not announced yet: everything buffered was written before the
		// announcement, so it is all data of this entry.
		if err := consume(r.br.Buffered()); err != nil {
			return n, err
		}
	}
}

// parseCentral parses the central directory and end records and compares
// them with the local entries.
func (s *Stream) parseCentral(r *posReader, entries []StreamEntry, cdStart int64) error {
	le := binary.LittleEndian
	count := 0
	first := true
	for {
		if !first {
			sigb, err := r.full(4)
			if err != nil {
				return fail("central directory truncated")
			}
			if sig := le.Uint32(sigb); sig != sigCentral {
				if count != len(entries) {
					return fail("central directory has %d entries, the zip %d", count, len(entries))
				}
				return s.parseEnd(r, sig, uint64(count), cdStart)
			}
		}
		first = false
		h, err := r.full(42)
		if err != nil {
			return fail("central directory truncated")
		}
		if count >= len(entries) {
			return fail("central directory has more entries than the zip")
		}
		e := entries[count]
		method := le.Uint16(h[6:])
		crc := le.Uint32(h[12:])
		csize, usize := uint64(le.Uint32(h[16:])), uint64(le.Uint32(h[20:]))
		nameLen, extraLen, commentLen := int(le.Uint16(h[24:])), int(le.Uint16(h[26:])), int(le.Uint16(h[28:]))
		offset := uint64(le.Uint32(h[38:]))
		name, err := r.full(nameLen)
		if err != nil {
			return fail("central directory truncated")
		}
		extra, err := r.full(extraLen)
		if err != nil {
			return fail("central directory truncated")
		}
		origExtra := extra
		if _, err := r.full(commentLen); err != nil {
			return fail("central directory truncated")
		}
		// zip64 extra field: the 0xFFFFFFFF values follow in order.
		for len(extra) >= 4 {
			tag, size := le.Uint16(extra), int(le.Uint16(extra[2:]))
			if 4+size > len(extra) {
				return fail("bad extra field")
			}
			body := extra[4 : 4+size]
			if tag == 0x0001 {
				for _, f := range []*uint64{&usize, &csize, &offset} {
					if *f == 0xffffffff && len(body) >= 8 {
						*f = le.Uint64(body)
						body = body[8:]
					}
				}
			}
			extra = extra[4+size:]
		}
		if le.Uint16(h[2:]) != e.version || le.Uint16(h[4:]) != e.flags || le.Uint16(h[8:]) != e.mtime || le.Uint16(h[10:]) != e.mdate ||
			le.Uint16(h[30:]) != 0 || !bytes.HasPrefix(origExtra, e.extra) {
			return fail("central directory entry %q does not match its local header", name)
		}
		if string(name) != e.Name || method != e.Method || offset != uint64(e.HeaderOffset) || //nolint:gosec // offsets are positive
			crc != e.CRC32 || csize != e.CompressedSize || usize != e.Size {
			return fail("central directory entry %q does not match its local entry", name)
		}
		count++
	}
}

func (s *Stream) parseEnd(r *posReader, sig uint32, count uint64, cdStart int64) error {
	le := binary.LittleEndian
	cdEnd := r.pos - 4
	cdSize := uint64(cdEnd - cdStart) //nolint:gosec // positions only grow
	if sig == sigZip64End {
		h, err := r.full(52)
		if err != nil {
			return fail("zip64 end record truncated")
		}
		size := le.Uint64(h)
		if size < 44 || size > 1<<16 {
			return fail("zip64 end record size")
		}
		if _, err := r.full(int(size) - 44); err != nil {
			return fail("zip64 end record truncated")
		}
		if le.Uint32(h[12:]) != 0 || le.Uint32(h[16:]) != 0 || le.Uint64(h[20:]) != count || le.Uint64(h[28:]) != count || le.Uint64(h[36:]) != cdSize || le.Uint64(h[44:]) != uint64(cdStart) { //nolint:gosec // positive
			return fail("zip64 end record does not match")
		}
		loc, err := r.full(20)
		if err != nil || le.Uint32(loc) != sigZip64Locate || le.Uint32(loc[4:]) != 0 || le.Uint64(loc[8:]) != uint64(cdEnd) || le.Uint32(loc[16:]) != 1 { //nolint:gosec // positive
			return fail("zip64 locator does not match")
		}
		sigb, err := r.full(4)
		if err != nil {
			return fail("end record missing")
		}
		sig = le.Uint32(sigb)
	}
	if sig != sigEnd {
		return fail("end record missing")
	}
	h, err := r.full(18)
	if err != nil {
		return fail("end record truncated")
	}
	if le.Uint16(h[0:]) != 0 || le.Uint16(h[2:]) != 0 || le.Uint16(h[4:]) != le.Uint16(h[6:]) {
		return fail("end record does not match")
	}
	n16 := uint64(le.Uint16(h[6:]))
	size32, off32 := uint64(le.Uint32(h[8:])), uint64(le.Uint32(h[12:]))
	if (n16 != 0xffff && n16 != count) || (size32 != 0xffffffff && size32 != cdSize) || (off32 != 0xffffffff && off32 != uint64(cdStart)) { //nolint:gosec // positive
		return fail("end record does not match")
	}
	if commentLen := int(le.Uint16(h[16:])); commentLen != 0 {
		return fail("unexpected zip comment")
	}
	return nil
}
