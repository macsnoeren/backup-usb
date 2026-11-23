package app

import (
	"fmt"
	"os"
	"path"
	"time"

	"filippo.io/age"

	"github.com/macsnoeren/backup-usb/internal/archive"
	"github.com/macsnoeren/backup-usb/internal/crypt"
	"github.com/macsnoeren/backup-usb/internal/fsx"
)

// IndexName returns the index file name of a tag.
func IndexName(tag string, encrypted bool) string {
	if encrypted {
		return tag + ".index" + crypt.Suffix
	}
	return tag + archive.IndexSuffix
}

// WriteIndex writes the index of an archive next to it, encrypted for the
// recipients if any, via a synced part file.
func WriteIndex(root *os.Root, folder, tag, period, archiveName string, created time.Time, res *archive.Result, recipients []age.Recipient) (err error) {
	rel := path.Join(folder, IndexName(tag, len(recipients) > 0))
	part := rel + fsx.PartSuffix
	f, err := fsx.CreateExclusive(root, part, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = root.Remove(part)
		}
	}()
	ix := archive.NewIndex(tag, period, archiveName, created, res)
	if len(recipients) > 0 {
		aw, aerr := age.Encrypt(f, recipients...)
		if aerr != nil {
			_ = f.Close()
			return fmt.Errorf("encrypt index: %w", aerr)
		}
		err = ix.Encode(aw)
		if cerr := aw.Close(); err == nil {
			err = cerr
		}
	} else {
		err = ix.Encode(f)
	}
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return fmt.Errorf("write index: %w", err)
	}
	return fsx.RootRename(root, part, rel)
}

// ReadIndex reads the index of a tag in a folder. For an encrypted archive
// only the encrypted index counts (decrypted with ids): a plaintext index
// next to an encrypted archive could have been planted by anyone. A
// missing index gives fs.ErrNotExist.
func ReadIndex(root *os.Root, folder, tag string, encrypted bool, ids []age.Identity) (*archive.Index, error) {
	f, err := fsx.OpenRegular(root, path.Join(folder, IndexName(tag, encrypted)))
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }() // read-only
	if !encrypted {
		return archive.DecodeIndex(f)
	}
	if len(ids) == 0 {
		return nil, errNeedKey
	}
	r, err := age.Decrypt(f, ids...)
	if err != nil {
		return nil, Errorf(KindDecrypt, err, "err.decrypt", path.Join(folder, IndexName(tag, true)))
	}
	return archive.DecodeIndex(r)
}

var errNeedKey = Errorf(KindUsage, nil, "err.needKey")
