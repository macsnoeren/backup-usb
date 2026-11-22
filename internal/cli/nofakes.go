//go:build !fakes

package cli

// FakesEnabled reports whether this is a test build with fake disks and a
// fake clock.
const FakesEnabled = false

// applyFakes does nothing in production builds: USB_BACKUP_FAKE_DISKS and
// USB_BACKUP_FAKE_NOW are ignored.
func applyFakes(*Env) error { return nil }
