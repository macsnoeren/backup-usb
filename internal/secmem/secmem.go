package secmem

import (
	"crypto/subtle"
	"runtime"
	"unsafe"
)

// Buffer holds a secret (password or private key) in memory that is locked
// against swapping where the platform allows it, and is overwritten with
// zeros by Wipe. Never convert the contents to a Go string except at the
// boundary of an API that requires one (see UnsafeString).
type Buffer struct {
	b      []byte
	locked bool
}

// New returns a zeroed, locked buffer of n bytes.
func New(n int) *Buffer {
	b := &Buffer{b: make([]byte, n)}
	if n > 0 {
		b.locked = lock(b.b) == nil
	}
	return b
}

// From copies src into a new locked buffer and wipes src.
func From(src []byte) *Buffer {
	b := New(len(src))
	copy(b.b, src)
	Wipe(src)
	return b
}

// Bytes returns the contents. The slice stays valid until Wipe.
func (b *Buffer) Bytes() []byte {
	if b == nil {
		return nil
	}
	return b.b
}

// Len returns the length.
func (b *Buffer) Len() int {
	if b == nil {
		return 0
	}
	return len(b.b)
}

// Locked reports whether the memory is locked against swapping.
func (b *Buffer) Locked() bool { return b != nil && b.locked }

// Wipe overwrites the contents with zeros and unlocks the memory. It is safe
// to call more than once and on nil.
func (b *Buffer) Wipe() {
	if b == nil || b.b == nil {
		return
	}
	Wipe(b.b)
	if b.locked {
		_ = unlock(b.b)
		b.locked = false
	}
	b.b = b.b[:0]
}

// Truncate shortens the buffer to n bytes, wiping the rest.
func (b *Buffer) Truncate(n int) {
	if b == nil || n >= len(b.b) {
		return
	}
	Wipe(b.b[n:])
	b.b = b.b[:n]
}

// Equal compares two secrets in constant time.
func (b *Buffer) Equal(o *Buffer) bool {
	return subtle.ConstantTimeCompare(b.Bytes(), o.Bytes()) == 1
}

// Wipe overwrites b with zeros.
func Wipe(b []byte) {
	clear(b)
	runtime.KeepAlive(b)
}

// UnsafeString returns a string that shares memory with b, without a copy.
// It exists only for APIs that take a secret as a string (filippo.io/age):
// the string becomes all zeros when the buffer is wiped, so it must not be
// used afterwards and must never be stored, logged or printed.
func UnsafeString(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return unsafe.String(unsafe.SliceData(b), len(b)) //nolint:gosec // deliberate: avoid an unwipeable copy of a secret
}
