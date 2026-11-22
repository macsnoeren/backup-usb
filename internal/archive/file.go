package archive

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/macsnoeren/backup-usb/internal/fsx"
)

// WriteFile archives src to dest+".part" and syncs it. The caller renames
// the part file after verification. On any error the part file is removed.
func WriteFile(ctx context.Context, src, dest string, opts Options) (*Result, error) {
	part := dest + fsx.PartSuffix
	f, err := os.OpenFile(part, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, fmt.Errorf("create %s: %w", filepath.Base(part), err)
	}
	res, err := Write(ctx, src, f, opts)
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		if rerr := os.Remove(part); rerr != nil && !errors.Is(rerr, os.ErrNotExist) {
			err = errors.Join(err, rerr)
		}
		return nil, err
	}
	return res, nil
}
