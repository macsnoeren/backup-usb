package app

import (
	"bufio"
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"filippo.io/age"

	"github.com/macsnoeren/backup-usb/internal/config"
	"github.com/macsnoeren/backup-usb/internal/fsx"
	"github.com/macsnoeren/backup-usb/internal/secmem"
	"github.com/macsnoeren/backup-usb/internal/vault"
)

// Recipients is the parsed recipients file.
type Recipients struct {
	// Lines are the public keys, in file order.
	Lines []string
	// Operational is the recipient marked as operational (first one if no
	// marks are present).
	Operational string
	// Breakglass is the recipient marked as break-glass.
	Breakglass string
}

// Fingerprints returns the fingerprints of all recipients.
func (r *Recipients) Fingerprints() []string {
	out := make([]string, len(r.Lines))
	for i, l := range r.Lines {
		out[i] = vault.Fingerprint(l)
	}
	return out
}

// AgeRecipients parses the recipients for encryption. They must all be
// post-quantum: age refuses a mix with classic keys.
func (r *Recipients) AgeRecipients() ([]age.Recipient, error) {
	out := make([]age.Recipient, 0, len(r.Lines))
	for _, l := range r.Lines {
		if !strings.HasPrefix(l, "age1pq1") {
			return nil, fmt.Errorf("recipient %s… is not post-quantum", l[:min(len(l), 12)])
		}
		rc, err := age.ParseHybridRecipient(l)
		if err != nil {
			return nil, fmt.Errorf("invalid recipient: %w", err)
		}
		out = append(out, rc)
	}
	if len(out) == 0 {
		return nil, errors.New("no recipients")
	}
	return out, nil
}

// ReadRecipients reads the recipients file. Comment lines "# operational"
// and "# break-glass" mark the line that follows.
func ReadRecipients(path string) (*Recipients, error) {
	data, err := config.ReadLimited(path, 256<<10)
	if err != nil {
		return nil, err
	}
	r := &Recipients{}
	mark := ""
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 8192), 64<<10)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		switch {
		case line == "":
		case strings.HasPrefix(line, "#"):
			c := strings.TrimSpace(strings.TrimPrefix(line, "#"))
			switch {
			case strings.HasPrefix(c, "operational"):
				mark = "op"
			case strings.HasPrefix(c, "break-glass"):
				mark = "bg"
			}
		default:
			r.Lines = append(r.Lines, line)
			switch mark {
			case "op":
				r.Operational = line
			case "bg":
				r.Breakglass = line
			}
			mark = ""
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read recipients: %w", err)
	}
	if len(r.Lines) == 0 {
		return nil, errors.New("recipients file has no keys")
	}
	if r.Operational == "" {
		r.Operational = r.Lines[0]
	}
	return r, nil
}

// RecipientsFile formats the recipients file.
func RecipientsFile(opID, operational, breakglass string, extra ...string) []byte {
	var b strings.Builder
	b.WriteString("# usb-backup recipients: public keys, one per line. Not secret.\n")
	fmt.Fprintf(&b, "# operational %s (fingerprint %s)\n%s\n", opID, vault.Fingerprint(operational), operational)
	fmt.Fprintf(&b, "# break-glass (fingerprint %s)\n%s\n", vault.Fingerprint(breakglass), breakglass)
	for _, e := range extra {
		fmt.Fprintf(&b, "# additional (fingerprint %s)\n%s\n", vault.Fingerprint(e), e)
	}
	return []byte(b.String())
}

// ReadSigningPub reads usb-backup.signing.pub.
func ReadSigningPub(dir string) (ed25519.PublicKey, error) {
	data, err := config.ReadLimited(filepath.Join(dir, config.SigningPubFile), 4096)
	if err != nil {
		return nil, err
	}
	pub, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(data)))
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return nil, errors.New("invalid signing public key")
	}
	return pub, nil
}

// generateIdentity creates a post-quantum key pair and returns the private
// key in a locked buffer and the public key.
func generateIdentity() (*secmem.Buffer, string, error) {
	id, err := age.GenerateHybridIdentity()
	if err != nil {
		return nil, "", fmt.Errorf("generate key: %w", err)
	}
	// age only hands out the private key as a string; copy it into locked
	// memory right away (the string itself cannot be wiped).
	return secmem.From([]byte(id.String())), id.Recipient().String(), nil
}

// KeysInitOptions configure keys init.
type KeysInitOptions struct {
	ConfigDir string
	Config    *config.Config
	VaultPath string
	Password  *secmem.Buffer
	// BreakglassOut receives the break-glass key once; it is synced before
	// anything else is saved.
	BreakglassOut interface {
		io.Writer
		Sync() error
	}
	Force bool
	Now   time.Time
}

// KeysInitResult describes the created keys (public data only).
type KeysInitResult struct {
	Vault                 string `json:"vault"`
	Recipients            string `json:"recipients"`
	OperationalKey        string `json:"operationalKey"`
	OperationalRecipient  string `json:"operationalRecipient"`
	BreakglassFingerprint string `json:"breakglassFingerprint"`
	BreakglassRecipient   string `json:"breakglassRecipient"`
	SigningPublic         string `json:"signingPublic"`
}

// KeysInit creates the operational and break-glass key pairs and the
// signing key pair, writes the recipients file, the signing public key and
// the vault, and writes the break-glass private key to BreakglassOut.
func KeysInit(o KeysInitOptions) (*KeysInitResult, error) {
	recPath := o.Config.RecipientsPath(o.ConfigDir)
	if !o.Force {
		for _, p := range []string{o.VaultPath, recPath, filepath.Join(o.ConfigDir, config.SigningPubFile)} {
			if _, err := os.Stat(p); err == nil {
				return nil, Errorf(KindUsage, nil, "err.keysExist", p)
			}
		}
	}
	op, opRcp, err := generateIdentity()
	if err != nil {
		return nil, Errorf(KindIO, err, "err.keygen")
	}
	defer op.Wipe()
	bg, bgRcp, err := generateIdentity()
	if err != nil {
		return nil, Errorf(KindIO, err, "err.keygen")
	}
	defer bg.Wipe()
	seed := secmem.New(ed25519.SeedSize)
	defer seed.Wipe()
	if _, err := rand.Read(seed.Bytes()); err != nil {
		return nil, Errorf(KindIO, err, "err.keygen")
	}
	v, err := vault.New(seed.Bytes(), bgRcp)
	if err != nil {
		return nil, Errorf(KindIO, err, "err.keygen")
	}
	defer v.Close()
	k, err := v.AddKey("k1", op.Bytes(), o.Now)
	if err != nil {
		return nil, Errorf(KindIO, err, "err.keygen")
	}

	// The break-glass key goes out first: if that fails, nothing is
	// written and the keys are gone, which is safe.
	if err := writeBreakglass(o.BreakglassOut, bg, bgRcp, o.Now); err != nil {
		return nil, Errorf(KindIO, err, "err.breakglassWrite")
	}
	if err := o.BreakglassOut.Sync(); err != nil {
		return nil, Errorf(KindIO, err, "err.breakglassWrite")
	}
	if err := os.MkdirAll(o.ConfigDir, 0o755); err != nil { //nolint:gosec // configuration folder is readable by everyone
		return nil, Errorf(KindIO, err, "err.write")
	}
	if err := v.Save(o.VaultPath, o.Password); err != nil {
		return nil, Errorf(KindIO, err, "err.write")
	}
	if err := fsx.WriteFileAtomic(recPath, RecipientsFile(k.ID, opRcp, bgRcp), 0o644); err != nil {
		return nil, Errorf(KindIO, err, "err.write")
	}
	pub, _ := v.SigningPublic()
	pubText := base64.StdEncoding.EncodeToString(pub)
	if err := fsx.WriteFileAtomic(filepath.Join(o.ConfigDir, config.SigningPubFile), []byte(pubText+"\n"), 0o644); err != nil {
		return nil, Errorf(KindIO, err, "err.write")
	}
	return &KeysInitResult{
		Vault: o.VaultPath, Recipients: recPath, OperationalKey: k.ID, OperationalRecipient: opRcp,
		BreakglassFingerprint: vault.Fingerprint(bgRcp), BreakglassRecipient: bgRcp, SigningPublic: pubText,
	}, nil
}

func writeBreakglass(w io.Writer, key *secmem.Buffer, recipient string, now time.Time) error {
	head := fmt.Sprintf("# USB Backup break-glass key, created %s.\n# Keep this offline (printed in a safe). Anyone with it can read every backup.\n# Restore without the app: age -d -i <this file> Data.zip.age > Data.zip\n# public key: %s\n", now.Format("2006-01-02"), recipient)
	if _, err := io.WriteString(w, head); err != nil {
		return err
	}
	if _, err := w.Write(key.Bytes()); err != nil {
		return err
	}
	_, err := w.Write([]byte("\n"))
	return err
}

// CreateSecretFile creates a new file for a secret with mode 0600; it never
// overwrites an existing file.
func CreateSecretFile(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if errors.Is(err, fs.ErrExist) {
		return nil, Errorf(KindUsage, err, "err.fileExists", path)
	}
	if err != nil {
		return nil, Errorf(KindIO, err, "err.write")
	}
	return f, nil
}
