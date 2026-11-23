package crypt

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"filippo.io/age"
)

func TestRewrap(t *testing.T) {
	dir := t.TempDir()
	plain := strings.Repeat("backup data ", 50000)
	var buf bytes.Buffer
	w, _ := age.Encrypt(&buf, recipients(t)...) // operational + break-glass
	w.Write([]byte(plain))
	w.Close()
	os.WriteFile(filepath.Join(dir, "Data.zip.age"), buf.Bytes(), 0o600)
	root, _ := os.OpenRoot(dir)
	defer root.Close()

	r2, _ := age.ParseHybridRecipient(testutilRecipient(t, "operational2"))
	bg, _ := age.ParseHybridRecipient(testutilRecipient(t, "breakglass"))
	op2 := identity(t, "operational2")
	if err := Rewrap(root, "Data.zip.age", []age.Identity{identity(t, "operational")}, []age.Recipient{r2, bg}, op2, ""); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "Data.zip.age"))
	open := func(name string) error {
		r, err := age.Decrypt(bytes.NewReader(data), identity(t, name))
		if err != nil {
			return err
		}
		b, _ := io.ReadAll(r)
		if string(b) != plain {
			t.Fatal("content changed")
		}
		return nil
	}
	if open("operational") == nil {
		t.Fatal("the old operational key still works")
	}
	if err := open("operational2"); err != nil {
		t.Fatal(err)
	}
	if err := open("breakglass"); err != nil {
		t.Fatal(err)
	}
	// The payload is byte-equal: only the header changed.
	oldPayload := buf.Bytes()[len(mustExtract(t, buf.Bytes())):]
	newPayload := data[len(mustExtract(t, data)):]
	if !bytes.Equal(oldPayload, newPayload) {
		t.Fatal("payload changed")
	}
	if e, _ := os.ReadDir(dir); len(e) != 1 {
		t.Fatal("part file left")
	}

	// With a key that does not fit: ErrDecrypt, file unchanged.
	err := Rewrap(root, "Data.zip.age", []age.Identity{identity(t, "operational")}, []age.Recipient{r2, bg}, op2, "")
	if !errors.Is(err, ErrDecrypt) {
		t.Fatalf("err = %v", err)
	}
	// A check key that is not among the new recipients: refused, unchanged.
	err = Rewrap(root, "Data.zip.age", []age.Identity{op2}, []age.Recipient{bg}, op2, "")
	if err == nil {
		t.Fatal("rewrap succeeded without a working new key")
	}
	after, _ := os.ReadFile(filepath.Join(dir, "Data.zip.age"))
	if !bytes.Equal(after, data) {
		t.Fatal("failed rewrap changed the file")
	}
	if e, _ := os.ReadDir(dir); len(e) != 1 {
		t.Fatal("part file left after failure")
	}
}

func TestRewrapRefusesDamagedFile(t *testing.T) {
	dir := t.TempDir()
	var buf bytes.Buffer
	w, _ := age.Encrypt(&buf, recipients(t)...)
	w.Write([]byte(strings.Repeat("x", 100000)))
	w.Close()
	data := buf.Bytes()
	sum := sha256.Sum256(data)
	data[len(data)/2] ^= 1 // bit rot after the backup was written
	os.WriteFile(filepath.Join(dir, "a.zip.age"), data, 0o600)
	root, _ := os.OpenRoot(dir)
	defer root.Close()
	r2, _ := age.ParseHybridRecipient(testutilRecipient(t, "operational2"))
	err := Rewrap(root, "a.zip.age", []age.Identity{identity(t, "operational")}, []age.Recipient{r2}, identity(t, "operational2"), hex.EncodeToString(sum[:]))
	if !errors.Is(err, ErrDamaged) {
		t.Fatalf("damaged file re-wrapped: %v", err)
	}
	if after, _ := os.ReadFile(filepath.Join(dir, "a.zip.age")); !bytes.Equal(after, data) {
		t.Fatal("file changed")
	}
}
