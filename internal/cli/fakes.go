//go:build fakes

package cli

import (
	"fmt"
	"time"

	"github.com/macsnoeren/backup-usb/internal/volume"
)

// FakesEnabled reports whether this is a test build with fake disks and a
// fake clock.
const FakesEnabled = true

// applyFakes replaces the disk finder and the clock from the environment.
// Only compiled into test binaries (build tag "fakes").
func applyFakes(e *Env) error {
	if p := e.Getenv("USB_BACKUP_FAKE_DISKS"); p != "" {
		e.Finder = volume.NewFakeFinder(p)
	}
	if s := e.Getenv("USB_BACKUP_FAKE_NOW"); s != "" {
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			return fmt.Errorf("USB_BACKUP_FAKE_NOW: %w", err)
		}
		start := time.Now()
		e.Now = func() time.Time { return t.Add(time.Since(start)) }
	}
	if d := e.Getenv("USB_BACKUP_FAKE_CONFIG"); d != "" {
		e.ConfigDir = d
	}
	return nil
}
