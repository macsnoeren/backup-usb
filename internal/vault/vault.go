package vault

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"filippo.io/age"

	"github.com/macsnoeren/backup-usb/internal/secmem"
)

// ScryptWorkFactor is the scrypt log2(N) for new vaults. Tests lower it.
var ScryptWorkFactor = 18

// MaxSize bounds the decrypted vault.
const MaxSize = 1 << 20

// Key statuses.
const (
	StatusActive  = "active"
	StatusRetired = "retired"
)

// Errors.
var (
	ErrWrongPassword = errors.New("wrong vault password or damaged vault")
	ErrNotFound      = errors.New("vault not found")
	ErrFormat        = errors.New("vault format not recognised")
)

// Key is the public information about one encryption key. The private key
// is held separately in wipeable memory.
type Key struct {
	ID           string   `json:"id"`
	Fingerprint  string   `json:"fingerprint"`
	Recipient    string   `json:"recipient"`
	Status       string   `json:"status"`
	Created      string   `json:"created"`
	Retired      string   `json:"retired,omitempty"`
	PendingDisks []string `json:"pendingDisks"`
}

// Meta is everything in the vault that is not secret.
type Meta struct {
	Version int   `json:"version"`
	Keys    []Key `json:"keys"`
	// SigningPublic is the Ed25519 public key (base64).
	SigningPublic string `json:"signingPublic"`
	// BreakglassRecipient and BreakglassFingerprint describe the break-glass
	// key; its private key is never in the vault.
	BreakglassRecipient   string `json:"breakglassRecipient"`
	BreakglassFingerprint string `json:"breakglassFingerprint"`
}

// Vault is an opened vault. Close wipes all secrets.
type Vault struct {
	Meta       Meta
	identities []*secmem.Buffer // "AGE-SECRET-KEY-PQ-1..." per key, same order as Meta.Keys
	signing    *secmem.Buffer   // Ed25519 seed
}

// Fingerprint is the first 16 hex characters of the SHA-256 of a public
// key string.
func Fingerprint(recipient string) string {
	sum := sha256.Sum256([]byte(recipient))
	return hex.EncodeToString(sum[:])[:16]
}

const (
	header       = "# usb-backup vault v1\n# An age identity file: \"age -d -i recovery.vault file.age\" works with it.\n"
	metaPrefix   = "# meta: "
	signPrefix   = "# signing: "
	secretPrefix = "AGE-SECRET-KEY-" //nolint:gosec // a line prefix, not a credential
)

// New returns an empty vault with the given signing seed (copied; the
// caller wipes its own copy).
func New(signingSeed []byte, breakglassRecipient string) (*Vault, error) {
	if len(signingSeed) != ed25519.SeedSize {
		return nil, errors.New("invalid signing seed")
	}
	v := &Vault{signing: secmem.New(ed25519.SeedSize)}
	copy(v.signing.Bytes(), signingSeed)
	priv := ed25519.NewKeyFromSeed(v.signing.Bytes())
	defer secmem.Wipe(priv)
	pub, ok := priv.Public().(ed25519.PublicKey)
	if !ok {
		return nil, errors.New("ed25519 public key")
	}
	v.Meta = Meta{
		Version:               1,
		SigningPublic:         base64.StdEncoding.EncodeToString(pub),
		BreakglassRecipient:   breakglassRecipient,
		BreakglassFingerprint: Fingerprint(breakglassRecipient),
	}
	return v, nil
}

// AddKey adds a private key (an age identity line, copied) as the new
// active key; existing active keys become retired.
func (v *Vault) AddKey(id string, identity []byte, now time.Time) (*Key, error) {
	parsed, err := age.ParseHybridIdentity(secmem.UnsafeString(identity))
	if err != nil {
		return nil, fmt.Errorf("invalid key: %w", err)
	}
	recipient := parsed.Recipient().String()
	for i := range v.Meta.Keys {
		if v.Meta.Keys[i].Status == StatusActive {
			v.Meta.Keys[i].Status = StatusRetired
			v.Meta.Keys[i].Retired = now.Format("2006-01-02")
		}
	}
	k := Key{ID: id, Fingerprint: Fingerprint(recipient), Recipient: recipient, Status: StatusActive, Created: now.Format("2006-01-02"), PendingDisks: []string{}}
	v.Meta.Keys = append(v.Meta.Keys, k)
	b := secmem.New(len(identity))
	copy(b.Bytes(), identity)
	v.identities = append(v.identities, b)
	return &v.Meta.Keys[len(v.Meta.Keys)-1], nil
}

// RemoveKey deletes a key and wipes its private key.
func (v *Vault) RemoveKey(id string) error {
	for i, k := range v.Meta.Keys {
		if k.ID == id {
			if k.Status == StatusActive {
				return errors.New("the active key cannot be removed")
			}
			v.identities[i].Wipe()
			v.Meta.Keys = append(v.Meta.Keys[:i], v.Meta.Keys[i+1:]...)
			v.identities = append(v.identities[:i], v.identities[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("no key %q", id)
}

// Active returns the active key.
func (v *Vault) Active() *Key {
	for i := range v.Meta.Keys {
		if v.Meta.Keys[i].Status == StatusActive {
			return &v.Meta.Keys[i]
		}
	}
	return nil
}

// Key returns the key with the given ID.
func (v *Vault) Key(id string) *Key {
	for i := range v.Meta.Keys {
		if v.Meta.Keys[i].ID == id {
			return &v.Meta.Keys[i]
		}
	}
	return nil
}

// Identities parses the private keys for decryption. The parsed values are
// held by the age library and cannot be wiped; keep their lifetime short.
func (v *Vault) Identities() ([]age.Identity, error) {
	out := make([]age.Identity, 0, len(v.identities))
	for _, b := range v.identities {
		id, err := age.ParseHybridIdentity(secmem.UnsafeString(b.Bytes()))
		if err != nil {
			return nil, fmt.Errorf("vault key: %w", err)
		}
		out = append(out, id)
	}
	return out, nil
}

// Identity parses the private key of one key.
func (v *Vault) Identity(id string) (age.Identity, error) {
	for i, k := range v.Meta.Keys {
		if k.ID == id {
			return age.ParseHybridIdentity(secmem.UnsafeString(v.identities[i].Bytes()))
		}
	}
	return nil, fmt.Errorf("no key %q", id)
}

// SigningKey returns the Ed25519 private key in a locked buffer; the caller
// wipes it.
func (v *Vault) SigningKey() *secmem.Buffer {
	priv := ed25519.NewKeyFromSeed(v.signing.Bytes())
	return secmem.From(priv)
}

// SigningPublic returns the Ed25519 public key.
func (v *Vault) SigningPublic() (ed25519.PublicKey, error) {
	pub, err := base64.StdEncoding.DecodeString(v.Meta.SigningPublic)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return nil, errors.New("invalid signing public key in vault")
	}
	return pub, nil
}

// Close wipes every secret.
func (v *Vault) Close() {
	if v == nil {
		return
	}
	for _, b := range v.identities {
		b.Wipe()
	}
	v.signing.Wipe()
}

// plaintext builds the vault content in one pre-sized locked buffer.
func (v *Vault) plaintext() (*secmem.Buffer, error) {
	meta, err := json.Marshal(v.Meta)
	if err != nil {
		return nil, fmt.Errorf("encode vault: %w", err)
	}
	if bytes.ContainsAny(meta, "\r\n") {
		return nil, errors.New("encode vault: newline in metadata")
	}
	seedLen := base64.StdEncoding.EncodedLen(v.signing.Len())
	n := len(header) + len(metaPrefix) + len(meta) + 1 + len(signPrefix) + seedLen + 1
	for _, b := range v.identities {
		n += b.Len() + 1
	}
	out := secmem.New(n)
	b := out.Bytes()
	i := copy(b, header)
	i += copy(b[i:], metaPrefix)
	i += copy(b[i:], meta)
	b[i] = '\n'
	i++
	i += copy(b[i:], signPrefix)
	base64.StdEncoding.Encode(b[i:i+seedLen], v.signing.Bytes())
	i += seedLen
	b[i] = '\n'
	i++
	for _, id := range v.identities {
		i += copy(b[i:], id.Bytes())
		b[i] = '\n'
		i++
	}
	return out, nil
}

// Save encrypts the vault with the password (age scrypt) and writes it
// atomically with mode 0600.
func (v *Vault) Save(path string, pw *secmem.Buffer) error {
	pt, err := v.plaintext()
	if err != nil {
		return err
	}
	defer pt.Wipe()
	r, err := age.NewScryptRecipient(secmem.UnsafeString(pw.Bytes()))
	if err != nil {
		return fmt.Errorf("vault password: %w", err)
	}
	r.SetWorkFactor(ScryptWorkFactor)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create vault folder: %w", err)
	}
	part := path + ".part"
	f, err := os.OpenFile(part, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if errors.Is(err, os.ErrExist) {
		if err := os.Remove(part); err != nil {
			return fmt.Errorf("remove old vault part: %w", err)
		}
		f, err = os.OpenFile(part, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	}
	if err != nil {
		return fmt.Errorf("create vault: %w", err)
	}
	w, err := age.Encrypt(f, r)
	if err == nil {
		_, err = w.Write(pt.Bytes())
		if cerr := w.Close(); err == nil {
			err = cerr
		}
	}
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(part, path)
	}
	if err != nil {
		_ = os.Remove(part)
		return fmt.Errorf("write vault: %w", err)
	}
	return nil
}

// Open decrypts the vault at path with the password.
func Open(path string, pw *secmem.Buffer) (*Vault, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("open vault: %w", err)
	}
	defer func() { _ = f.Close() }() // read-only
	id, err := age.NewScryptIdentity(secmem.UnsafeString(pw.Bytes()))
	if err != nil {
		return nil, ErrWrongPassword
	}
	r, err := age.Decrypt(io.LimitReader(f, MaxSize*2), id)
	if err != nil {
		var noMatch *age.NoIdentityMatchError
		if errors.As(err, &noMatch) {
			return nil, ErrWrongPassword
		}
		return nil, fmt.Errorf("%w: %w", ErrWrongPassword, err)
	}
	pt := secmem.New(MaxSize + 1)
	defer pt.Wipe()
	n, err := io.ReadFull(r, pt.Bytes())
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, fmt.Errorf("%w: %w", ErrWrongPassword, err)
	}
	if n > MaxSize {
		return nil, ErrFormat
	}
	return parse(pt.Bytes()[:n])
}

// parse reads the plaintext without copying secrets into strings.
func parse(pt []byte) (*Vault, error) {
	v := &Vault{}
	ok := false
	for len(pt) > 0 {
		line := pt
		if i := bytes.IndexByte(pt, '\n'); i >= 0 {
			line, pt = pt[:i], pt[i+1:]
		} else {
			pt = nil
		}
		line = bytes.TrimRight(line, "\r")
		switch {
		case bytes.HasPrefix(line, []byte(metaPrefix)):
			if err := json.Unmarshal(line[len(metaPrefix):], &v.Meta); err != nil {
				v.Close()
				return nil, ErrFormat
			}
			ok = true
		case bytes.HasPrefix(line, []byte(signPrefix)):
			enc := line[len(signPrefix):]
			v.signing = secmem.New(base64.StdEncoding.DecodedLen(len(enc)))
			n, err := base64.StdEncoding.Decode(v.signing.Bytes(), enc)
			if err != nil || n != ed25519.SeedSize {
				v.Close()
				return nil, ErrFormat
			}
			v.signing.Truncate(n)
		case bytes.HasPrefix(line, []byte(secretPrefix)):
			b := secmem.New(len(line))
			copy(b.Bytes(), line)
			v.identities = append(v.identities, b)
		}
	}
	if !ok || v.signing == nil || len(v.identities) != len(v.Meta.Keys) || v.Meta.Version != 1 {
		v.Close()
		return nil, ErrFormat
	}
	return v, nil
}

// ChangePassword re-encrypts the vault at path with a new password. The
// backups are not touched.
func ChangePassword(path string, oldPw, newPw *secmem.Buffer) error {
	v, err := Open(path, oldPw)
	if err != nil {
		return err
	}
	defer v.Close()
	return v.Save(path, newPw)
}
