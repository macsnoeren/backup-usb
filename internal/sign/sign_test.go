package sign

import (
	"crypto/ed25519"
	"errors"
	"strings"
	"testing"

	"github.com/macsnoeren/backup-usb/internal/testutil"
)

func keys(t *testing.T, name string) (ed25519.PrivateKey, ed25519.PublicKey) {
	priv := ed25519.NewKeyFromSeed(testutil.TestSigningSeed(t, name))
	return priv, priv.Public().(ed25519.PublicKey)
}

func TestSigFile(t *testing.T) {
	priv, pub := keys(t, "signing")
	_, other := keys(t, "signing-other")
	data := []byte(`{"keepPerDisk": 25}` + "\n")
	sig, err := SigFile(priv, PurposeConfig, data)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifySigFile(pub, PurposeConfig, data, sig); err != nil {
		t.Fatal(err)
	}
	for i := range data {
		bad := append([]byte(nil), data...)
		bad[i] ^= 1
		if err := VerifySigFile(pub, PurposeConfig, bad, sig); !errors.Is(err, ErrBadSignature) {
			t.Fatalf("changed byte %d accepted", i)
		}
	}
	if err := VerifySigFile(other, PurposeConfig, data, sig); !errors.Is(err, ErrBadSignature) {
		t.Fatal("other key accepted")
	}
	if err := VerifySigFile(pub, PurposeRegistry, data, sig); !errors.Is(err, ErrBadSignature) {
		t.Fatal("signature reused for another purpose")
	}
	if err := VerifySigFile(pub, PurposeConfig, data, []byte("garbage")); !errors.Is(err, ErrBadSignature) {
		t.Fatal("garbage accepted")
	}
}

func TestJSONField(t *testing.T) {
	priv, pub := keys(t, "signing")
	_, other := keys(t, "signing-other")
	obj := []byte(`{"name":"BACKUP1","id":"3f9c2b1e-7a4d-4e8b-9c21-5d6e7f8a9b0c","volumeSerial":"A41C-77E2","version":1}`)
	signed, err := SignJSON(priv, PurposeDiskID, obj)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyJSON(pub, PurposeDiskID, signed); err != nil {
		t.Fatal(err)
	}
	// Re-formatting does not matter, one changed character does.
	compact := strings.ReplaceAll(strings.ReplaceAll(string(signed), "\n", ""), "  ", "")
	if err := VerifyJSON(pub, PurposeDiskID, []byte(compact)); err != nil {
		t.Fatal("canonical form not used:", err)
	}
	for _, change := range []string{"BACKUP2", "A41C-77E3"} {
		bad := strings.Replace(string(signed), map[string]string{"BACKUP2": "BACKUP1", "A41C-77E3": "A41C-77E2"}[change], change, 1)
		if err := VerifyJSON(pub, PurposeDiskID, []byte(bad)); !errors.Is(err, ErrBadSignature) {
			t.Fatalf("change to %s accepted", change)
		}
	}
	if err := VerifyJSON(other, PurposeDiskID, signed); !errors.Is(err, ErrBadSignature) {
		t.Fatal("other key accepted")
	}
	if err := VerifyJSON(pub, PurposeDiskID, obj); !errors.Is(err, ErrBadSignature) {
		t.Fatal("unsigned accepted")
	}
	// An added field is a change too.
	added := strings.Replace(string(signed), `"name"`, `"extra": 1, "name"`, 1)
	if err := VerifyJSON(pub, PurposeDiskID, []byte(added)); !errors.Is(err, ErrBadSignature) {
		t.Fatal("added field accepted")
	}
}

func FuzzVerifyJSON(f *testing.F) {
	f.Add([]byte(`{"a":1,"signature":"x"}`))
	_, pub := ed25519.GenerateKey(nil)
	f.Fuzz(func(t *testing.T, data []byte) {
		if VerifyJSON(pub, PurposeDiskID, data) == nil {
			t.Fatal("random data verified")
		}
	})
}
