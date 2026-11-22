package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"filippo.io/age"

	"github.com/macsnoeren/backup-usb/internal/testutil"
)

func encJob(t *testing.T) EncJob {
	j := zipJob(t)
	j.Rel = "2026/47/Data.zip.age"
	var rs []age.Recipient
	for _, n := range []string{"operational", "breakglass"} {
		r, _ := age.ParseHybridRecipient(testutil.TestRecipient(t, n))
		rs = append(rs, r)
	}
	return EncJob{ZipJob: j, Recipients: rs}
}

func TestWriteVerifiedEncrypted(t *testing.T) {
	job := encJob(t)
	out, err := WriteVerifiedEncrypted(context.Background(), job)
	if err != nil {
		t.Fatal(err)
	}
	got := testutil.Tree(t, weekDir(job.ZipJob))
	if len(got) != 2 || got["Data.zip.age.sha256"] != out.SHA256+"  Data.zip.age\n" {
		t.Fatalf("week folder %v", got)
	}
	if strings.Contains(got["Data.zip.age"], "PK\x03\x04") {
		t.Fatal("zip on disk")
	}
}

func TestChangedByteOnDiskIsFound(t *testing.T) {
	job := encJob(t)
	testHookAfterWrite = func(part string) error {
		p := filepath.Join(job.Root.Name(), part)
		data, _ := os.ReadFile(p)
		data[len(data)/2] ^= 0x01
		return os.WriteFile(p, data, 0o600)
	}
	defer func() { testHookAfterWrite = nil }()
	_, err := WriteVerifiedEncrypted(context.Background(), job)
	if KindOf(err) != KindVerify {
		t.Fatalf("err = %v", err)
	}
	if got := testutil.Tree(t, weekDir(job.ZipJob)); len(got) != 0 {
		t.Fatalf("left behind %v", got)
	}
}
