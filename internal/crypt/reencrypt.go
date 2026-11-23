package crypt

import (
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"io"
	"os"

	"filippo.io/age"

	"github.com/macsnoeren/backup-usb/internal/fsx"
)

// Reencrypt decrypts rel with oldIDs and encrypts it again for recipients
// with a new file key (keys rotate --full). The part file is decrypted
// with check and must give the same plaintext before it replaces rel. The
// plaintext only passes through memory.
func Reencrypt(root *os.Root, rel string, oldIDs []age.Identity, recipients []age.Recipient, check age.Identity) (err error) {
	f, err := fsx.OpenRegular(root, rel)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }() // read-only
	r, err := age.Decrypt(f, oldIDs...)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrDecrypt, err)
	}
	part := rel + fsx.PartSuffix
	out, err := fsx.CreateExclusive(root, part, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = root.Remove(part)
		}
	}()
	aw, err := age.Encrypt(out, recipients...)
	if err != nil {
		_ = out.Close()
		return fmt.Errorf("encrypt: %w", err)
	}
	h := sha256.New()
	_, cerr := io.Copy(io.MultiWriter(aw, h), r)
	if cerr != nil {
		_ = out.Close()
		return fmt.Errorf("%w: %w", ErrDecrypt, cerr)
	}
	werr := aw.Close()
	serr := out.Sync()
	clerr := out.Close()
	if err := errors.Join(werr, serr, clerr); err != nil {
		return fmt.Errorf("write %s: %w", part, err)
	}
	want := h.Sum(nil)
	pf, err := fsx.OpenRegular(root, part)
	if err != nil {
		return err
	}
	defer func() { _ = pf.Close() }() // read-only
	pr, err := age.Decrypt(pf, check)
	if err != nil {
		return fmt.Errorf("re-encrypted file does not open with the new key: %w", err)
	}
	h2 := sha256.New()
	if _, err := io.Copy(h2, pr); err != nil {
		return fmt.Errorf("read back: %w", err)
	}
	if subtle.ConstantTimeCompare(h2.Sum(nil), want) != 1 {
		return errors.New("re-encrypted content differs")
	}
	return fsx.RootRename(root, part, rel)
}

// FileKeyOf returns the file key of an age file (tests and checks only).
func FileKeyOf(root *os.Root, rel string, ids ...age.Identity) ([]byte, error) {
	f, err := fsx.OpenRegular(root, rel)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }() // read-only
	hdr, _, err := ReadHeader(f)
	if err != nil {
		return nil, err
	}
	return age.DecryptHeader(hdr.Marshal(), ids...)
}

// OpensWith reports whether the header of rel opens with id.
func OpensWith(root *os.Root, rel string, id age.Identity) bool {
	fk, err := FileKeyOf(root, rel, id)
	if err != nil {
		return false
	}
	for i := range fk {
		fk[i] = 0
	}
	return true
}
