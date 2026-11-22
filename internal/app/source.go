package app

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"filippo.io/age"

	"github.com/macsnoeren/backup-usb/internal/archive"
	"github.com/macsnoeren/backup-usb/internal/crypt"
	"github.com/macsnoeren/backup-usb/internal/fsx"
)

// openedSource is one archive opened for reading, with its index.
type openedSource struct {
	file      *os.File
	zip       io.ReaderAt // decrypted when encrypted
	zipSize   int64
	index     *archive.Index
	fromIndex bool
	manifest  map[string]string
	encrypted bool
}

func (s *openedSource) Close() {
	if s.file != nil {
		_ = s.file.Close()
	}
}

// openSource opens the archive of tag in folder. The index is used when
// present; without it (T5.5) the zip's central directory is read through
// the decryption, and the manifest gives the SHA-256 of every file.
func openSource(root *os.Root, folder, tag string, ids []age.Identity) (*openedSource, error) {
	names, err := archivesIn(root, folder, tag)
	if err != nil {
		return nil, Errorf(KindIO, err, "err.read")
	}
	var name string
	for _, n := range names {
		if m := archiveFileRe.FindStringSubmatch(n); m != nil && strings.EqualFold(m[1], tag) {
			name, tag = n, m[1]
			break
		}
	}
	if name == "" {
		return nil, Errorf(KindUsage, nil, "err.noArchive", tag, folder)
	}
	encrypted := strings.HasSuffix(name, crypt.Suffix)
	if encrypted && len(ids) == 0 {
		return nil, errNeedKey
	}
	f, err := fsx.OpenRegular(root, path.Join(folder, name))
	if err != nil {
		return nil, Errorf(KindIO, err, "err.read")
	}
	src := &openedSource{file: f, encrypted: encrypted}
	st, err := f.Stat()
	if err != nil {
		src.Close()
		return nil, Errorf(KindIO, err, "err.read")
	}
	if encrypted {
		ra, size, err := age.DecryptReaderAt(f, st.Size(), ids...)
		if err != nil {
			src.Close()
			return nil, Errorf(KindDecrypt, err, "err.decrypt", path.Join(folder, name))
		}
		src.zip, src.zipSize = ra, size
	} else {
		src.zip, src.zipSize = f, st.Size()
	}
	// The archive itself opened with the key, so an index that does not
	// read (missing, damaged, wrong size) is not fatal: fall back to the
	// zip's own central directory and manifest.
	ix, err := ReadIndex(root, folder, tag, ids)
	if err == nil && ix.ZipSize == src.zipSize && strings.EqualFold(ix.Tag, tag) {
		src.index, src.fromIndex = ix, true
		return src, nil
	}
	if err := src.indexFromZip(tag, name); err != nil {
		src.Close()
		return nil, err
	}
	return src, nil
}

// indexFromZip builds an index from the central directory and the
// manifest of the archive.
func (s *openedSource) indexFromZip(tag, name string) error {
	zr, err := zip.NewReader(s.zip, s.zipSize)
	if err != nil {
		return Errorf(KindVerify, err, "err.verify", name)
	}
	ix := &archive.Index{Version: 1, Tag: tag, Archive: name, ZipSize: s.zipSize}
	for _, f := range zr.File {
		if f.Name == archive.ManifestName {
			rc, err := f.Open()
			if err != nil {
				return Errorf(KindVerify, err, "err.verify", name)
			}
			m, err := archive.ParseManifest(io.LimitReader(rc, archive.MaxManifestSize))
			_ = rc.Close()
			if err != nil {
				return Errorf(KindVerify, err, "err.verify", name)
			}
			s.manifest = m
			continue
		}
		off, err := f.DataOffset()
		if err != nil {
			return Errorf(KindVerify, err, "err.verify", name)
		}
		e := archive.Entry{
			Path: f.Name, Dir: f.FileInfo().IsDir(), Size: f.UncompressedSize64, Modified: f.Modified,
			Mode: uint32(f.Mode().Perm()), CRC32: f.CRC32, Method: f.Method, DataOffset: off, CompressedSize: f.CompressedSize64,
		}
		if f.Mode()&os.ModeSymlink != 0 {
			e.Link = "?"
		}
		ix.Entries = append(ix.Entries, e)
	}
	if s.manifest == nil {
		return Errorf(KindVerify, errors.New("manifest missing"), "err.verify", name)
	}
	for i := range ix.Entries {
		ix.Entries[i].SHA256 = s.manifest[ix.Entries[i].Path]
	}
	s.index = ix
	return nil
}

// entryReader returns the uncompressed data of an index entry, read
// directly at its offset in the (decrypted) zip.
func (s *openedSource) entryReader(e archive.Entry) (io.ReadCloser, error) {
	if e.DataOffset < 0 || e.DataOffset > s.zipSize || e.CompressedSize > uint64(s.zipSize-e.DataOffset) { //nolint:gosec // bounded above
		return nil, fmt.Errorf("entry %q outside the archive", e.Path)
	}
	sec := io.NewSectionReader(s.zip, e.DataOffset, int64(e.CompressedSize)) //nolint:gosec // bounded above
	switch e.Method {
	case zip.Store:
		return io.NopCloser(sec), nil
	case zip.Deflate:
		return flateReader(sec), nil
	}
	return nil, fmt.Errorf("entry %q: unsupported method %d", e.Path, e.Method)
}
