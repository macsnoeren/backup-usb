// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Maurice Snoeren
//
// This file is part of USB Backup.
//
// USB Backup is free software: you can redistribute it and/or modify it under
// the terms of the GNU General Public License as published by the Free Software
// Foundation, either version 3 of the License, or (at your option) any later
// version.
//
// USB Backup is distributed in the hope that it will be useful, but WITHOUT ANY
// WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A
// PARTICULAR PURPOSE. See the GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License along with
// USB Backup. If not, see <https://www.gnu.org/licenses/>.

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
// wipes it), or nil when the session is over. An expired session is ended
// here too, so a click after the timeout cannot bring it back.
func (s *Session) Password() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pw == nil {
		return nil
	}
	if s.now().Sub(s.last) >= s.timeout {
		s.pw.Wipe()
		s.pw = nil
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
