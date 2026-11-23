package crypt

import (
	"bufio"
	"bytes"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"

	"filippo.io/age"
)

// This is the only hand-written part of the age format, following the
// public specification (https://age-encryption.org/v1): the header can be
// read and written so a file can be re-wrapped for new recipients without
// touching its payload. It is tested against filippo.io/age.

const (
	intro         = "age-encryption.org/v1\n"
	stanzaPrefix  = "-> "
	footerPrefix  = "---"
	columnsPerRow = 64
	// Limits for untrusted headers.
	maxHeaderSize = 1 << 20
	maxStanzas    = 64
	maxArgs       = 16
	maxBodySize   = 64 << 10
)

var b64 = base64.RawStdEncoding.Strict()

// ErrHeader means the age header is not well formed.
var ErrHeader = errors.New("invalid age header")

// Header is a parsed age header.
type Header struct {
	Stanzas []*age.Stanza
	MAC     []byte
}

func headerErr(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrHeader, fmt.Sprintf(format, args...))
}

func validArg(a string) bool {
	if a == "" {
		return false
	}
	for i := 0; i < len(a); i++ {
		if a[i] < 33 || a[i] > 126 {
			return false
		}
	}
	return true
}

// ParseHeader reads an age header from r. After it returns, r is positioned
// at the start of the payload (the nonce).
func ParseHeader(r *bufio.Reader) (*Header, error) {
	read := 0
	line := func() (string, error) {
		s, err := r.ReadString('\n')
		read += len(s)
		if read > maxHeaderSize {
			return "", headerErr("header too large")
		}
		if err != nil {
			return "", headerErr("header truncated")
		}
		return s, nil
	}
	first, err := line()
	if err != nil {
		return nil, err
	}
	if first != intro {
		return nil, headerErr("not an age v1 file")
	}
	h := &Header{}
	for {
		l, err := line()
		if err != nil {
			return nil, err
		}
		if strings.HasPrefix(l, footerPrefix) {
			mac, ok := strings.CutPrefix(strings.TrimSuffix(l, "\n"), footerPrefix+" ")
			if !ok {
				return nil, headerErr("malformed MAC line")
			}
			m, err := b64.DecodeString(mac)
			if err != nil || len(m) != 32 {
				return nil, headerErr("malformed MAC")
			}
			h.MAC = m
			if len(h.Stanzas) == 0 {
				return nil, headerErr("no recipients")
			}
			return h, nil
		}
		if !strings.HasPrefix(l, stanzaPrefix) {
			return nil, headerErr("unexpected line")
		}
		if len(h.Stanzas) >= maxStanzas {
			return nil, headerErr("too many recipients")
		}
		fields := strings.Split(strings.TrimSuffix(l[len(stanzaPrefix):], "\n"), " ")
		if len(fields) > maxArgs+1 {
			return nil, headerErr("too many stanza arguments")
		}
		for _, f := range fields {
			if !validArg(f) {
				return nil, headerErr("invalid stanza argument")
			}
		}
		s := &age.Stanza{Type: fields[0], Args: fields[1:]}
		var body []byte
		for {
			bl, err := line()
			if err != nil {
				return nil, err
			}
			bl = strings.TrimSuffix(bl, "\n")
			if len(bl) > columnsPerRow {
				return nil, headerErr("body line too long")
			}
			chunk, err := b64.DecodeString(bl)
			if err != nil {
				return nil, headerErr("invalid body encoding")
			}
			body = append(body, chunk...)
			if len(body) > maxBodySize {
				return nil, headerErr("stanza body too large")
			}
			if len(bl) < columnsPerRow {
				break
			}
		}
		s.Body = body
		h.Stanzas = append(h.Stanzas, s)
	}
}

// macInput is the header up to and including "---", which the MAC covers.
func (h *Header) macInput() []byte {
	var b bytes.Buffer
	b.WriteString(intro)
	for _, s := range h.Stanzas {
		b.WriteString(stanzaPrefix + s.Type)
		for _, a := range s.Args {
			b.WriteString(" " + a)
		}
		b.WriteString("\n")
		enc := b64.EncodeToString(s.Body)
		for len(enc) >= columnsPerRow {
			b.WriteString(enc[:columnsPerRow] + "\n")
			enc = enc[columnsPerRow:]
		}
		b.WriteString(enc + "\n")
	}
	b.WriteString(footerPrefix)
	return b.Bytes()
}

// Marshal returns the complete header including the MAC line.
func (h *Header) Marshal() []byte {
	out := h.macInput()
	return append(append(out, ' '), []byte(b64.EncodeToString(h.MAC)+"\n")...)
}

func headerMAC(fileKey, input []byte) ([]byte, error) {
	key, err := hkdf.Key(sha256.New, fileKey, nil, "header", 32)
	if err != nil {
		return nil, err
	}
	m := hmac.New(sha256.New, key)
	m.Write(input)
	return m.Sum(nil), nil
}

// SetMAC computes the header MAC with the file key.
func (h *Header) SetMAC(fileKey []byte) error {
	mac, err := headerMAC(fileKey, h.macInput())
	if err != nil {
		return err
	}
	h.MAC = mac
	return nil
}

// CheckMAC verifies the header MAC with the file key.
func (h *Header) CheckMAC(fileKey []byte) bool {
	mac, err := headerMAC(fileKey, h.macInput())
	return err == nil && hmac.Equal(mac, h.MAC)
}

// ReadHeader parses the header at the start of an age file and returns the
// header and a reader for the payload.
func ReadHeader(r io.Reader) (*Header, io.Reader, error) {
	br := bufio.NewReaderSize(r, 64<<10)
	h, err := ParseHeader(br)
	if err != nil {
		return nil, nil, err
	}
	return h, br, nil
}
