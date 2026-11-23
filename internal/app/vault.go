// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Maurice Snoeren
//
// This file is part of USB Backup.
//
// USB Backup is free software: you can redistribute it and/or modify it under
// the terms of the GNU General Public License as published by the Free Software
// Foundation, either version 3 of the License, or (at your option) any later
// version.
//
// USB Backup is distributed in the hope that it will be useful, but WITHOUT ANY
// WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A
// PARTICULAR PURPOSE. See the GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License along with
// USB Backup. If not, see <https://www.gnu.org/licenses/>.

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
