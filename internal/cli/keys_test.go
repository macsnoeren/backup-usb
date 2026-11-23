package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"filippo.io/age"

	"github.com/macsnoeren/backup-usb/internal/app"
	"github.com/macsnoeren/backup-usb/internal/config"
	"github.com/macsnoeren/backup-usb/internal/sign"
	"github.com/macsnoeren/backup-usb/internal/vault"
)

func init() { vault.ScryptWorkFactor = 10 }

const testPassword = "correct horse battery staple"

// keysInit runs keys init in w and returns the break-glass key file.
func (w *world) keysInit() string {
	w.t.Helper()
	bg := filepath.Join(w.t.TempDir(), "breakglass.txt")
	w.stdin = testPassword + "\n" + testPassword + "\n"
	defer func() { w.stdin = "" }()
	w.mustRun(0, "keys", "init", "--vault", w.vaultPath(), "--breakglass-out", bg, "--password-stdin")
	w.resignIDFiles()
	w.bgKey = bg
	return bg
}

// resignIDFiles signs ID files that tests wrote before keys existed, as a
// re-enroll would.
func (w *world) resignIDFiles() {
	v, err := vault.Open(w.vaultPath(), secmemFrom(testPassword))
	if err != nil {
		w.t.Fatal(err)
	}
	defer v.Close()
	key := v.SigningKey()
	defer key.Wipe()
	for _, name := range []string{"BACKUP1", "BACKUP2"} {
		p := w.diskFile(name, "Backup/disk-id.json")
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		signed, err := sign.SignJSON(key.Bytes(), sign.PurposeDiskID, data)
		if err != nil {
			w.t.Fatal(err)
		}
		os.Chmod(p, 0o644)
		os.WriteFile(p, signed, 0o644)
	}
}

func (w *world) vaultPath() string { return filepath.Join(w.cfgDir, "user", "recovery.vault") }

func TestKeysInit(t *testing.T) {
	w := newWorld(t)
	bg := w.keysInit()
	rec, err := app.ReadRecipients(filepath.Join(w.cfgDir, config.DefaultRecipients))
	if err != nil {
		t.Fatal(err)
	}
	if len(rec.Lines) != 2 || rec.Operational == rec.Breakglass {
		t.Fatalf("recipients %+v", rec)
	}
	for _, r := range rec.Lines {
		if !strings.HasPrefix(r, "age1pq1") {
			t.Fatalf("recipient %q is not post-quantum", r[:10])
		}
	}
	if _, err := app.ReadSigningPub(w.cfgDir); err != nil {
		t.Fatal(err)
	}
	st, _ := os.Stat(bg)
	if st.Mode().Perm() != 0o600 {
		t.Errorf("break-glass file mode %v", st.Mode().Perm())
	}
	bgData, _ := os.ReadFile(bg)
	ids, err := age.ParseIdentities(bytes.NewReader(bgData))
	if err != nil || len(ids) != 1 {
		t.Fatalf("break-glass file: %v", err)
	}
	if ids[0].(*age.HybridIdentity).Recipient().String() != rec.Breakglass {
		t.Fatal("break-glass key does not match the recipient")
	}
	var secret string
	for _, l := range strings.Split(string(bgData), "\n") {
		if strings.HasPrefix(l, "AGE-SECRET-KEY-") {
			secret = l
		}
	}
	// The break-glass private key is nowhere else: not in the
	// configuration folder, not in the vault, not in any output or log.
	filepath.Walk(w.cfgDir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		data, _ := os.ReadFile(p)
		if bytes.Contains(data, []byte(secret)) || bytes.Contains(data, []byte(secret[15:40])) {
			t.Errorf("break-glass key found in %s", p)
		}
		return nil
	})
	v, err := vault.Open(w.vaultPath(), secmemFrom(testPassword))
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	if v.Meta.BreakglassRecipient != rec.Breakglass || len(v.Meta.Keys) != 1 {
		t.Fatal("vault metadata")
	}
	// Again without --force: refused, nothing changed.
	w.stdin = testPassword + "\n" + testPassword + "\n"
	code, _, errOut := w.run("keys", "init", "--vault", w.vaultPath(), "--breakglass-out", filepath.Join(t.TempDir(), "x"), "--password-stdin")
	if code != 64 || !strings.Contains(errOut, "already exists") {
		t.Fatalf("second init: %d %s", code, errOut)
	}
}

func TestKeysInitOutputHasNoSecret(t *testing.T) {
	w := newWorld(t)
	bg := filepath.Join(t.TempDir(), "bg.txt")
	w.stdin = testPassword + "\n" + testPassword + "\n"
	code, out, errOut := w.run("keys", "init", "--vault", w.vaultPath(), "--breakglass-out", bg, "--password-stdin", "--json")
	if code != 0 {
		t.Fatal(errOut)
	}
	var res app.KeysInitResult
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{out, errOut} {
		if strings.Contains(s, "AGE-SECRET-KEY") || strings.Contains(s, testPassword) {
			t.Fatal("secret in output")
		}
	}
}

func TestKeysInitRefusals(t *testing.T) {
	w := newWorld(t)
	bg := filepath.Join(t.TempDir(), "bg.txt")
	w.stdin = "short\nshort\n"
	if code, _, _ := w.run("keys", "init", "--vault", w.vaultPath(), "--breakglass-out", bg, "--password-stdin"); code != 64 {
		t.Fatal("short password accepted")
	}
	if _, err := os.Stat(bg); !os.IsNotExist(err) {
		t.Fatal("break-glass file created although init failed")
	}
	w.stdin = testPassword + "\n" + testPassword + "\n"
	if code, _, _ := w.run("keys", "init", "--vault", w.vaultPath(), "--password-stdin"); code != 64 {
		t.Fatal("missing --breakglass-out accepted")
	}
	os.WriteFile(bg, []byte("existing"), 0o600)
	if code, _, _ := w.run("keys", "init", "--vault", w.vaultPath(), "--breakglass-out", bg, "--password-stdin"); code != 64 {
		t.Fatal("existing break-glass file overwritten")
	}
}

func TestKeysPasswd(t *testing.T) {
	w := newWorld(t)
	w.keysInit()
	w.stdin = "wrong password here\n"
	if code, _, errOut := w.run("keys", "passwd", "--vault", w.vaultPath(), "--password-stdin"); code != 13 || !strings.Contains(errOut, "wrong vault password") {
		t.Fatalf("wrong password: %d %s", code, errOut)
	}
	w.stdin = testPassword + "\nanother long password\nanother long password\n"
	w.mustRun(0, "keys", "passwd", "--vault", w.vaultPath(), "--password-stdin")
	if _, err := vault.Open(w.vaultPath(), secmemFrom("another long password")); err != nil {
		t.Fatal(err)
	}
}
