package app

import (
	"context"
	"errors"
	"os"

	"github.com/macsnoeren/backup-usb/internal/fsx"

	"github.com/macsnoeren/backup-usb/internal/volume"
)

// Space estimates.
const (
	// SpaceMargin is added to every estimate.
	SpaceMargin = 100 << 20
	// FAT32MaxFile is the largest file FAT32 can hold.
	FAT32MaxFile = 1<<32 - 1
)

// SpaceNeed is what a backup is expected to write.
type SpaceNeed struct {
	// SourceBytes are the bytes of each source folder.
	SourceBytes []uint64
	// MinFreePercent is the margin in percent.
	MinFreePercent int
}

// Required returns the bytes needed: the sum of all sources plus
// MinFreePercent plus 100 MB.
func (n SpaceNeed) Required() uint64 {
	var total uint64
	for _, b := range n.SourceBytes {
		total += b
	}
	return total + total*uint64(max(n.MinFreePercent, 0))/100 + SpaceMargin
}

// Largest returns the largest single source, an upper bound for its zip.
func (n SpaceNeed) Largest() uint64 {
	var m uint64
	for _, b := range n.SourceBytes {
		m = max(m, b)
	}
	return m
}

// CheckFileSystem refuses FAT32 when a zip of more than 4 GB is expected.
func CheckFileSystem(v volume.Volume, need SpaceNeed) error {
	if volume.IsFAT32(v.FileSystem) && need.Largest() > FAT32MaxFile {
		return Errorf(KindDiskCheck, nil, "err.fat32", v.Label)
	}
	return nil
}

// CheckSpace compares the free space with the estimate.
func CheckSpace(free uint64, need SpaceNeed) error {
	if req := need.Required(); free < req {
		return Errorf(KindDiskCheck, nil, "err.space", humanBytes(req), humanBytes(free))
	}
	return nil
}

// CheckWritable creates, syncs and removes a test file in the backup
// folder, through its os.Root and exclusively, so a link planted on the
// disk is never followed.
func CheckWritable(root *os.Root) error {
	const name = ".usb-backup-write-test"
	f, err := fsx.CreateExclusive(root, name, 0o600)
	if err != nil {
		return Errorf(KindDiskCheck, err, "err.notWritable")
	}
	_, werr := f.Write([]byte("usb-backup write test\n"))
	serr := f.Sync()
	cerr := f.Close()
	rerr := root.Remove(name)
	if err := errors.Join(werr, serr, cerr, rerr); err != nil {
		return Errorf(KindDiskCheck, err, "err.notWritable")
	}
	return nil
}

// CheckDisk runs the file system, space and writability checks.
func CheckDisk(ctx context.Context, f volume.Finder, v volume.Volume, root *os.Root, need SpaceNeed) error {
	if err := CheckFileSystem(v, need); err != nil {
		return err
	}
	free, _, err := f.Space(ctx, v)
	if err != nil {
		return Errorf(KindDiskCheck, err, "err.spaceUnknown")
	}
	if err := CheckSpace(free, need); err != nil {
		return err
	}
	return CheckWritable(root)
}
