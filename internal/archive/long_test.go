//go:build long

package archive

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// TestLargeFileZip64 archives a sparse file larger than 4 GB.
func TestLargeFileZip64(t *testing.T) {
	src := t.TempDir()
	const size = 4<<30 + 12345
	f, err := os.Create(filepath.Join(src, "big.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(size); err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteAt([]byte("end"), size-3); err != nil {
		t.Fatal(err)
	}
	f.Close()
	dest := filepath.Join(t.TempDir(), "big.zip")
	opts := defaultOpts()
	opts.CompressionLevel = 1
	res, err := WriteFileTo(t, src, dest, opts)
	if err != nil {
		t.Fatal(err)
	}
	if res.Bytes != size {
		t.Fatalf("bytes %d", res.Bytes)
	}
	zr, err := zip.OpenReader(dest + ".part")
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	big := zr.File[0]
	if big.UncompressedSize64 != size {
		t.Fatalf("size in zip %d", big.UncompressedSize64)
	}
	rc, err := big.Open()
	if err != nil {
		t.Fatal(err)
	}
	n, err := io.Copy(io.Discard, rc)
	rc.Close()
	if err != nil || n != size {
		t.Fatalf("read back %d bytes: %v", n, err)
	}
}
