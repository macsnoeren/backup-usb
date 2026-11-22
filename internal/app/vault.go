package app

import (
	"errors"

	"github.com/macsnoeren/backup-usb/internal/secmem"
	"github.com/macsnoeren/backup-usb/internal/vault"
)

// OpenVault opens the vault and maps its errors to kinds: a wrong password
// or a damaged vault is KindDecrypt (exit code 13).
func OpenVault(path string, pw *secmem.Buffer) (*vault.Vault, error) {
	v, err := vault.Open(path, pw)
	switch {
	case err == nil:
		return v, nil
	case errors.Is(err, vault.ErrNotFound):
		return nil, Errorf(KindDecrypt, err, "err.vaultMissing", path)
	case errors.Is(err, vault.ErrWrongPassword):
		return nil, Errorf(KindDecrypt, nil, "err.vaultPassword")
	default:
		return nil, Errorf(KindDecrypt, err, "err.vaultDamaged")
	}
}
