package crypt

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"filippo.io/age"

	"github.com/macsnoeren/backup-usb/internal/archive"
	"github.com/macsnoeren/backup-usb/internal/testutil"
	"github.com/macsnoeren/backup-usb/internal/verify"
)

func recipients(t *testing.T) []age.Recipient {
	var out []age.Recipient
	for _, n := range []string{"operational", "breakglass"} {
		r, err := age.ParseHybridRecipient(testutil.TestRecipient(t, n))
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, r)
	}
	return out
}

func identity(t *testing.T, name string) age.Identity {
	id, err := age.ParseHybridIdentity(testutil.TestIdentity(t, name))
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// watchingRoot records every byte written to the disk folder.
func encryptTo(t *testing.T, src string) (string, *Result) {
	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	res, err := WriteEncrypted(context.Background(), src, root, "Data.zip.age", recipients(t), archive.Options{CompressionLevel: 6, StoreExtensions: []string{"jpg"}})
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(dir, "Data.zip.age.part"), res
}

func source(t *testing.T) string {
	src := t.TempDir()
	testutil.WriteFile(t, src, "secret-plans.txt", strings.Repeat("the quick brown fox ", 5000))
	testutil.WriteFile(t, src, "img/photo.jpg", strings.Repeat("JPEGDATA", 4000))
	testutil.WriteFile(t, src, "empty.txt", "")
	return src
}

func TestNoUnencryptedZipOnDisk(t *testing.T) {
	src := source(t)
	part, res := encryptTo(t, src)
	data, err := os.ReadFile(part)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(data, []byte("age-encryption.org/v1\n")) {
		t.Fatal("not an age file")
	}
	for _, leak := range []string{"PK\x03\x04", "secret-plans", "the quick brown fox", "JPEGDATA", archive.ManifestName} {
		if bytes.Contains(data, []byte(leak)) {
			t.Fatalf("plaintext %q on the disk", leak)
		}
	}
	// Only the part file exists in the folder.
	entries, _ := os.ReadDir(filepath.Dir(part))
	if len(entries) != 1 {
		t.Fatalf("folder has %d entries", len(entries))
	}
	if int64(len(data)) != res.Size {
		t.Fatal("size")
	}
}

func TestDecryptWithEitherKey(t *testing.T) {
	src := source(t)
	part, res := encryptTo(t, src)
	for _, key := range []string{"operational", "breakglass"} {
		f, _ := os.Open(part)
		r, err := age.Decrypt(f, identity(t, key))
		if err != nil {
			t.Fatalf("%s: %v", key, err)
		}
		zipData, err := io.ReadAll(r)
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
		if int64(len(zipData)) != res.Archive.ZipSize {
			t.Fatalf("%s: zip size %d, want %d", key, len(zipData), res.Archive.ZipSize)
		}
		if _, err := verify.Zip(context.Background(), bytes.NewReader(zipData), int64(len(zipData)), verify.Options{ExpectedFiles: res.Archive.Files, Entries: res.Archive.AllEntries()}); err != nil {
			t.Fatalf("%s: %v", key, err)
		}
	}
	f, _ := os.Open(part)
	defer f.Close()
	if _, err := age.Decrypt(f, identity(t, "other")); err == nil {
		t.Fatal("decrypted with an unrelated key")
	}
}

func TestMixedRecipientsRefused(t *testing.T) {
	x, _ := age.GenerateX25519Identity()
	root, _ := os.OpenRoot(t.TempDir())
	defer root.Close()
	_, err := WriteEncrypted(context.Background(), source(t), root, "Data.zip.age", append(recipients(t), x.Recipient()), archive.Options{})
	if err == nil {
		t.Fatal("classic and post-quantum recipients mixed")
	}
	if _, err := root.Stat("Data.zip.age.part"); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("part file left behind")
	}
}

func TestFailureRemovesPart(t *testing.T) {
	root, _ := os.OpenRoot(t.TempDir())
	defer root.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := WriteEncrypted(ctx, source(t), root, "Data.zip.age", recipients(t), archive.Options{}); err == nil {
		t.Fatal("cancelled write succeeded")
	}
	if _, err := root.Stat("Data.zip.age.part"); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("part file left behind")
	}
}
