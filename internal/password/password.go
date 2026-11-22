// Package password reads passwords from stdin or the console, straight into
// wipeable memory. Passwords never come from arguments or environment
// variables.
package password

import (
	"errors"
	"fmt"
	"io"
	"unicode/utf8"

	"github.com/macsnoeren/backup-usb/internal/secmem"
)

// MaxLen is the longest accepted password in bytes.
const MaxLen = 1024

// MinNewLen is the minimum length of a new vault password.
const MinNewLen = 12

// Errors.
var (
	ErrEmpty    = errors.New("empty password")
	ErrTooLong  = errors.New("password too long")
	ErrMismatch = errors.New("passwords do not match")
	ErrTooShort = fmt.Errorf("password shorter than %d characters", MinNewLen)
	ErrNoTTY    = errors.New("no console available for the password prompt; use --password-stdin")
)

// Source yields passwords.
type Source interface {
	// Password asks for a password with the given prompt.
	Password(prompt string) (*secmem.Buffer, error)
}

// ReadLine reads one line from r, byte by byte so nothing after the line is
// consumed, into a locked buffer. The line ending (\n or \r\n) is removed.
func ReadLine(r io.Reader) (*secmem.Buffer, error) {
	buf := secmem.New(MaxLen + 1)
	b := buf.Bytes()
	n := 0
	one := make([]byte, 1)
	defer secmem.Wipe(one)
	for {
		m, err := r.Read(one)
		if m == 1 {
			if one[0] == '\n' {
				break
			}
			if n == MaxLen {
				buf.Wipe()
				return nil, ErrTooLong
			}
			b[n] = one[0]
			n++
			continue
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			buf.Wipe()
			return nil, fmt.Errorf("read password: %w", err)
		}
	}
	if n > 0 && b[n-1] == '\r' {
		n--
	}
	buf.Truncate(n)
	if n == 0 {
		buf.Wipe()
		return nil, ErrEmpty
	}
	return buf, nil
}

// Reader reads passwords line by line from an io.Reader (--password-stdin
// or the pipe from the GUI).
type Reader struct{ R io.Reader }

// Password implements Source; the prompt is ignored.
func (s Reader) Password(string) (*secmem.Buffer, error) { return ReadLine(s.R) }

// NewPassword asks for a new password twice and checks length and match.
func NewPassword(src Source, prompt, again string) (*secmem.Buffer, error) {
	p1, err := src.Password(prompt)
	if err != nil {
		return nil, err
	}
	if utf8.RuneCount(p1.Bytes()) < MinNewLen {
		p1.Wipe()
		return nil, ErrTooShort
	}
	p2, err := src.Password(again)
	if err != nil {
		p1.Wipe()
		return nil, err
	}
	defer p2.Wipe()
	if !p1.Equal(p2) {
		p1.Wipe()
		return nil, ErrMismatch
	}
	return p1, nil
}
