package gui

import (
	"sync"
	"time"

	"github.com/macsnoeren/backup-usb/internal/secmem"
)

// Session is the operator session: the vault password in locked memory,
// forgotten after the timeout without activity.
type Session struct {
	mu      sync.Mutex
	pw      *secmem.Buffer
	last    time.Time
	timeout time.Duration
	now     func() time.Time
}

// NewSession starts a session that holds pw (taken over, wiped on end).
func NewSession(pw *secmem.Buffer, timeout time.Duration, now func() time.Time) *Session {
	return &Session{pw: pw, last: now(), timeout: timeout, now: now}
}

// Touch records activity.
func (s *Session) Touch() {
	s.mu.Lock()
	s.last = s.now()
	s.mu.Unlock()
}

// Expired ends the session when the timeout passed; it reports whether the
// session is over.
func (s *Session) Expired() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pw == nil {
		return true
	}
	if s.now().Sub(s.last) >= s.timeout {
		s.pw.Wipe()
		s.pw = nil
		return true
	}
	return false
}

// Password returns a copy of the password for one CLI call (the caller
// wipes it), or nil when the session is over.
func (s *Session) Password() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pw == nil {
		return nil
	}
	s.last = s.now()
	return append([]byte(nil), s.pw.Bytes()...)
}

// End wipes the password.
func (s *Session) End() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pw.Wipe()
	s.pw = nil
}
