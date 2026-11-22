package cli

import (
	"os"

	"github.com/macsnoeren/backup-usb/internal/vault"
)

func removeFile(p string) error { return os.Remove(p) }

func fingerprintOf(recipient string) string { return vault.Fingerprint(recipient) }
