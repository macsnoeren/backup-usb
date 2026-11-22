//go:build integration

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/macsnoeren/backup-usb/internal/app"
)

func TestVerifyWithKeys(t *testing.T) {
	w := newWorld(t)
	w.enrollUnsigned(map[string]string{"BACKUP1": "odd", "BACKUP2": "even"})
	bg := w.keysInit()
	w.mustRun(0, "run")
	w.clock.Advance(14 * 24 * time.Hour)
	w.mustRun(0, "run")

	// Without a key: only the hashes of the encrypted files.
	out, _ := w.mustRun(0, "verify", "--all")
	if strings.Count(out, "no key given") != 4 {
		t.Fatalf("verify without key:\n%s", out)
	}
	// With the break-glass key: unpacked, and recorded as tested.
	out, _ = w.mustRun(0, "verify", "--all", "--identity", bg)
	if strings.Count(out, "files unpacked and checked") != 4 || !strings.Contains(out, "break-glass key works") {
		t.Fatalf("verify with break-glass:\n%s", out)
	}
	if _, ok := app.BreakglassTested(w.cfgDir); !ok {
		t.Fatal("break-glass test not recorded")
	}
	// With the vault password.
	w.stdin = testPassword + "\n"
	out, _ = w.mustRun(0, "verify", "--week", "2026-47", "--password-stdin", "--vault", w.vaultPath())
	if strings.Count(out, "files unpacked") != 2 || !strings.Contains(out, "2026/47") {
		t.Fatalf("verify with vault:\n%s", out)
	}
	// Wrong password: 13.
	w.stdin = "not the password\n"
	w.mustRun(13, "verify", "--password-stdin", "--vault", w.vaultPath())
	// An unrelated key: 13.
	other := filepath.Join(t.TempDir(), "other.key")
	os.WriteFile(other, []byte(testIdentity(t, "other")+"\n"), 0o600)
	w.mustRun(13, "verify", "--identity", other)
}

func TestVerifyFindsDamage(t *testing.T) {
	w := newWorld(t)
	w.enrollUnsigned(map[string]string{"BACKUP1": "odd"})
	bg := w.keysInit()
	w.mustRun(0, "run")
	p := w.diskFile("BACKUP1", "Backup/2026/47/Data.zip.age")
	data, _ := os.ReadFile(p)
	data[len(data)/2] ^= 1
	os.WriteFile(p, data, 0o600)
	w.mustRun(12, "verify")
	// Fix the sha256 file so only the decryption can notice.
	sum, _, _ := hashOf(p)
	os.WriteFile(p+".sha256", []byte(sum+"  Data.zip.age\n"), 0o644)
	w.mustRun(0, "verify", "--tag", "Data")
	w.mustRun(13, "verify", "--tag", "Data", "--identity", bg)
}

func TestVerifyUnencrypted(t *testing.T) {
	w := newWorld(t)
	w.enrollUnsigned(map[string]string{"BACKUP1": "odd"})
	w.mustRun(0, "run")
	out, _ := w.mustRun(0, "verify")
	if strings.Count(out, "files unpacked") != 2 {
		t.Fatalf("%s", out)
	}
	w.mustRun(12, "verify", "--week", "2026-40")
}

func TestBreakglassNotRecordedWithoutDecryption(t *testing.T) {
	w := newWorld(t)
	w.enrollUnsigned(map[string]string{"BACKUP1": "odd"})
	w.mustRun(0, "run") // unencrypted week
	bg := w.keysInit()
	w.mustRun(0, "verify", "--identity", bg)
	if _, ok := app.BreakglassTested(w.cfgDir); ok {
		t.Fatal("break-glass recorded as tested without decrypting anything")
	}
}
