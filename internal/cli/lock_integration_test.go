//go:build integration

package cli

import (
	"path/filepath"
	"testing"

	"github.com/macsnoeren/backup-usb/internal/lock"
)

func TestSecondInstance(t *testing.T) {
	w := newWorld(t)
	w.enrollUnsigned(map[string]string{"BACKUP1": "odd"})
	release, err := lock.Acquire(filepath.Join(w.cfgDir, "test.lock"))
	if err != nil {
		t.Fatal(err)
	}
	w.mustRun(5, "run")
	w.mustRun(5, "verify")
	release()
	w.mustRun(0, "run")
}
