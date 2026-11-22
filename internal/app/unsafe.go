package app

import "github.com/macsnoeren/backup-usb/internal/secmem"

// unsafeString shares memory with a secret for APIs that need a string.
func unsafeString(b []byte) string { return secmem.UnsafeString(b) }
