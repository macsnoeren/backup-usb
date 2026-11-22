package cli

import (
	"errors"
	"os"

	"github.com/macsnoeren/backup-usb/internal/app"
	"github.com/macsnoeren/backup-usb/internal/vault"
)

func removeFile(p string) error { return os.Remove(p) }

func fingerprintOf(recipient string) string { return vault.Fingerprint(recipient) }

func asAppErr(err error, target **app.Error) bool { return errors.As(err, target) }
