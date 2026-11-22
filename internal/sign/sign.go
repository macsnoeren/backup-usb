package sign

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Purposes separate the signatures of different files, so a signature for
// one kind of file can never be reused for another.
const (
	PurposeConfig   = "config"
	PurposeRegistry = "registry"
	PurposeDiskID   = "disk-id"
)

// SignatureField is the JSON field that holds an embedded signature.
const SignatureField = "signature"

// ErrBadSignature means the data was changed or signed with another key.
var ErrBadSignature = errors.New("signature does not match")

func message(purpose string, data []byte) []byte {
	m := make([]byte, 0, 32+len(purpose)+len(data))
	m = append(m, "usb-backup signature v1\n"...)
	m = append(m, purpose...)
	m = append(m, '\n')
	return append(m, data...)
}

// Sign signs data for a purpose with an Ed25519 private key (64 bytes).
func Sign(priv []byte, purpose string, data []byte) ([]byte, error) {
	if len(priv) != ed25519.PrivateKeySize {
		return nil, errors.New("invalid signing key")
	}
	return ed25519.Sign(ed25519.PrivateKey(priv), message(purpose, data)), nil
}

// Verify checks a signature.
func Verify(pub ed25519.PublicKey, purpose string, data, sig []byte) error {
	if len(pub) != ed25519.PublicKeySize || len(sig) != ed25519.SignatureSize {
		return ErrBadSignature
	}
	if !ed25519.Verify(pub, message(purpose, data), sig) {
		return ErrBadSignature
	}
	return nil
}

// SigFile returns the content of a detached .sig file for data.
func SigFile(priv []byte, purpose string, data []byte) ([]byte, error) {
	sig, err := Sign(priv, purpose, data)
	if err != nil {
		return nil, err
	}
	return []byte(base64.StdEncoding.EncodeToString(sig) + "\n"), nil
}

// VerifySigFile checks data against the content of a .sig file. The exact
// bytes of data are signed, so any change (also whitespace) is refused.
func VerifySigFile(pub ed25519.PublicKey, purpose string, data, sigFile []byte) error {
	sig, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(sigFile)))
	if err != nil {
		return ErrBadSignature
	}
	return Verify(pub, purpose, data, sig)
}

// Canonical returns the canonical form of a JSON object without its
// signature field: keys sorted, no insignificant white space, numbers as
// written.
func Canonical(obj []byte) ([]byte, string, error) {
	dec := json.NewDecoder(bytes.NewReader(obj))
	dec.UseNumber()
	var m map[string]any
	if err := dec.Decode(&m); err != nil {
		return nil, "", fmt.Errorf("signed JSON: %w", err)
	}
	if dec.More() {
		return nil, "", errors.New("signed JSON: trailing data")
	}
	sig, _ := m[SignatureField].(string)
	delete(m, SignatureField)
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(m); err != nil {
		return nil, "", fmt.Errorf("signed JSON: %w", err)
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), sig, nil
}

// SignJSON signs a JSON object and returns it, indented, with the
// signature in its "signature" field.
func SignJSON(priv []byte, purpose string, obj []byte) ([]byte, error) {
	canon, _, err := Canonical(obj)
	if err != nil {
		return nil, err
	}
	sig, err := Sign(priv, purpose, canon)
	if err != nil {
		return nil, err
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(canon, &m); err != nil {
		return nil, err
	}
	enc, _ := json.Marshal(base64.StdEncoding.EncodeToString(sig))
	m[SignatureField] = enc
	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}

// VerifyJSON checks the embedded signature of a JSON object.
func VerifyJSON(pub ed25519.PublicKey, purpose string, obj []byte) error {
	canon, sigText, err := Canonical(obj)
	if err != nil || sigText == "" {
		return ErrBadSignature
	}
	sig, err := base64.StdEncoding.DecodeString(sigText)
	if err != nil {
		return ErrBadSignature
	}
	return Verify(pub, purpose, canon, sig)
}

// ParsePublicKey reads the content of usb-backup.signing.pub.
func ParsePublicKey(data []byte) (ed25519.PublicKey, error) {
	pub, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(data)))
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return nil, errors.New("invalid signing public key")
	}
	return pub, nil
}
