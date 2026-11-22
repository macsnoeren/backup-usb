package archive

import (
	"context"
	"errors"
	"os"

	"github.com/macsnoeren/backup-usb/internal/fsx"
)

// WriteFile archives src to rel+".part" inside root (the backup folder on a
// disk) and syncs it. The part file is created exclusively, so a link
// planted on the disk is never followed. The caller renames the part file
// after verification. On any error the part file is removed.
func WriteFile(ctx context.Context, src string, root *os.Root, rel string, opts Options) (*Result, error) {
	part := rel + fsx.PartSuffix
	f, err := fsx.CreateExclusive(root, part, 0o600)
	if err != nil {
		return nil, err
	}
	res, err := Write(ctx, src, f, opts)
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		if rerr := root.Remove(part); rerr != nil && !errors.Is(rerr, os.ErrNotExist) {
			err = errors.Join(err, rerr)
		}
		return nil, err
	}
	return res, nil
}
