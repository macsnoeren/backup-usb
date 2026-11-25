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

package secmem

import (
	"runtime"
	"testing"

	_ "github.com/macsnoeren/backup-usb/internal/testutil" // -update flag
)

func TestWipe(t *testing.T) {
	src := []byte("correct horse battery staple")
	b := From(src)
	for _, c := range src {
		if c != 0 {
			t.Fatal("source not wiped")
		}
	}
	if string(b.Bytes()) != "correct horse battery staple" {
		t.Fatal("copy wrong")
	}
	full := b.Bytes()[:cap(b.Bytes())]
	s := UnsafeString(b.Bytes())
	b.Wipe()
	for _, c := range full {
		if c != 0 {
			t.Fatal("buffer not all zeros after Wipe")
		}
	}
	for i := 0; i < len(s); i++ {
		if s[i] != 0 {
			t.Fatal("unsafe string still holds the secret")
		}
	}
	if b.Len() != 0 || b.Locked() {
		t.Fatal("state after wipe")
	}
	b.Wipe()
	var nb *Buffer
	nb.Wipe()
}

func TestTruncateAndEqual(t *testing.T) {
	b := From([]byte("secret\n"))
	full := b.Bytes()
	b.Truncate(6)
	if string(b.Bytes()) != "secret" || full[6] != 0 {
		t.Fatal("truncate")
	}
	if !b.Equal(From([]byte("secret"))) || b.Equal(From([]byte("secreT"))) {
		t.Fatal("equal")
	}
}

func TestProtectProcess(t *testing.T) {
	if err := ProtectProcess(); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "linux" {
		d, err := dumpable()
		if err != nil || d {
			t.Fatalf("still dumpable: %v %v", d, err)
		}
	}
}

// A shortened buffer unlocks everything it locked; with nothing left there
// is no address to pass to Windows (this crashed on an empty password).
func TestWipeAfterTruncate(t *testing.T) {
	old := unlockMem
	defer func() { unlockMem = old }()
	var unlocked []int
	unlockMem = func(b []byte) error { unlocked = append(unlocked, len(b)); return old(b) }
	for _, n := range []int{0, 3, 8} {
		b := New(8)
		b.locked = true
		b.Truncate(n)
		unlocked = nil
		b.Wipe()
		b.Wipe()
		if len(unlocked) != 1 || unlocked[0] != 8 {
			t.Errorf("truncated to %d: unlocked %v", n, unlocked)
		}
	}
	_ = lock(nil)
	_ = unlock(nil)
}
