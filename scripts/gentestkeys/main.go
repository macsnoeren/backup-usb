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
