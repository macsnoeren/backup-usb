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

// Command gentestkeys regenerates the fixed test keys in testdata/keys.
// These keys are for automated tests only and protect nothing.
//
//	go run ./scripts/gentestkeys testdata/keys
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"filippo.io/age"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: gentestkeys <dir>")
		os.Exit(64)
	}
	dir := os.Args[1]
	for _, name := range []string{"operational", "breakglass", "operational2", "other"} {
		id, err := age.GenerateHybridIdentity()
		if err != nil {
			panic(err)
		}
		content := fmt.Sprintf("# TEST KEY - DO NOT USE\n# public key: %s\n%s\n", id.Recipient(), id)
		if err := os.WriteFile(filepath.Join(dir, name+".key"), []byte(content), 0o600); err != nil {
			panic(err)
		}
	}
	for _, name := range []string{"signing", "signing-other"} {
		seed := make([]byte, ed25519.SeedSize)
		if _, err := rand.Read(seed); err != nil {
			panic(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name+".seed"), []byte(hex.EncodeToString(seed)+"\n"), 0o600); err != nil {
			panic(err)
		}
	}
}
