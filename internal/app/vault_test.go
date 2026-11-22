package app

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/macsnoeren/backup-usb/internal/secmem"
	"github.com/macsnoeren/backup-usb/internal/testutil"
	"github.com/macsnoeren/backup-usb/internal/vault"
)

func TestOpenVaultKinds(t *testing.T) {
	vault.ScryptWorkFactor = 10
	path := filepath.Join(t.TempDir(), "recovery.vault")
	v, _ := vault.New(testutil.TestSigningSeed(t, "signing"), testutil.TestRecipient(t, "breakglass"))
	v.AddKey("k1", []byte(testutil.TestIdentity(t, "operational")), time.Now())
	if err := v.Save(path, secmem.From([]byte("a good password"))); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenVault(path, secmem.From([]byte("a bad password!"))); KindOf(err) != KindDecrypt {
		t.Fatalf("wrong password: %v", err)
	}
	if _, err := OpenVault(path+"x", secmem.From([]byte("a good password"))); KindOf(err) != KindDecrypt {
		t.Fatalf("missing: %v", err)
	}
	got, err := OpenVault(path, secmem.From([]byte("a good password")))
	if err != nil {
		t.Fatal(err)
	}
	got.Close()
}
