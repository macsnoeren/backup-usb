package archive

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/macsnoeren/backup-usb/internal/fsx"
)

// Index file names: <tag>.index.gz, or <tag>.index.age when encrypted.
const (
	IndexSuffix = ".index.gz"
	// MaxIndexSize bounds a decompressed index read from a disk.
	MaxIndexSize = 128 << 20
)

// Index describes every entry of one archive, so a backup can be browsed
// and single files restored without reading the whole zip.
type Index struct {
	Version int       `json:"version"`
	Tag     string    `json:"tag"`
	Period  string    `json:"period"`
	Created time.Time `json:"created"`
	Archive string    `json:"archive"`
	ZipSize int64     `json:"zipSize"`
	Entries []Entry   `json:"entries"`
}

// NewIndex builds the index of a written archive.
func NewIndex(tag, period, archive string, created time.Time, res *Result) *Index {
	return &Index{Version: 1, Tag: tag, Period: period, Created: created, Archive: archive, ZipSize: res.ZipSize, Entries: res.Entries}
}

// Encode writes the index as gzip-compressed JSON.
func (ix *Index) Encode(w io.Writer) error {
	zw := gzip.NewWriter(w)
	enc := json.NewEncoder(zw)
	if err := enc.Encode(ix); err != nil {
		return fmt.Errorf("encode index: %w", err)
	}
	if err := zw.Close(); err != nil {
		return fmt.Errorf("encode index: %w", err)
	}
	return nil
}

// DecodeIndex reads an index. It is untrusted input: the size is bounded
// and every path is checked.
func DecodeIndex(r io.Reader) (*Index, error) {
	zr, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("index: %w", err)
	}
	defer func() { _ = zr.Close() }()
	lr := &io.LimitedReader{R: zr, N: MaxIndexSize + 1}
	data, err := io.ReadAll(lr)
	if err != nil {
		return nil, fmt.Errorf("index: %w", err)
	}
	if len(data) > MaxIndexSize {
		return nil, errors.New("index too large")
	}
	ix := &Index{}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(ix); err != nil {
		return nil, fmt.Errorf("index: %w", err)
	}
	if ix.Version != 1 {
		return nil, fmt.Errorf("index version %d not supported", ix.Version)
	}
	for _, e := range ix.Entries {
		if _, err := fsx.SafeRelOS(e.Path, "linux"); err != nil {
			return nil, fmt.Errorf("index: %w", err)
		}
		if e.DataOffset < e.HeaderOffset || e.HeaderOffset < 0 || e.DataOffset > ix.ZipSize {
			return nil, fmt.Errorf("index: invalid offsets for %q", e.Path)
		}
	}
	return ix, nil
}
