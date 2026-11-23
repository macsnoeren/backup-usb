package cli

import (
	"errors"
	"io"
	"os"

	"github.com/macsnoeren/backup-usb/internal/app"
	"github.com/macsnoeren/backup-usb/internal/msg"
	"github.com/macsnoeren/backup-usb/internal/secmem"
	"github.com/macsnoeren/backup-usb/internal/vault"
)

func removeFile(p string) error { return os.Remove(p) }

func fingerprintOf(recipient string) string { return vault.Fingerprint(recipient) }

func asAppErr(err error, target **app.Error) bool { return errors.As(err, target) }

// readSecretFile reads a key file straight into a locked buffer, without
// intermediate copies.
func readSecretFile(p string, maxSize int64) (*secmem.Buffer, error) {
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }() // read-only
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() || st.Size() > maxSize {
		return nil, errors.New("not a key file")
	}
	buf := secmem.New(int(st.Size()))
	if _, err := io.ReadFull(f, buf.Bytes()); err != nil {
		buf.Wipe()
		return nil, err
	}
	return buf, nil
}

func msgLang(s string) msg.Lang { return msg.Lang(s) }
