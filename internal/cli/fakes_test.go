package cli

import (
	"testing"
	"time"
)

func TestFakesOnlyInTestBuild(t *testing.T) {
	env := &Env{Now: time.Now, Getenv: func(k string) string {
		return map[string]string{"USB_BACKUP_FAKE_DISKS": "/nonexistent.json", "USB_BACKUP_FAKE_NOW": "2026-12-31T12:00:00Z"}[k]
	}}
	if err := applyFakes(env); err != nil {
		t.Fatal(err)
	}
	if FakesEnabled {
		if env.Finder == nil || env.Now().Year() != 2026 || env.Now().Month() != 12 {
			t.Fatal("fakes not applied in a fakes build")
		}
	} else if env.Finder != nil {
		t.Fatal("fakes applied in a production build")
	}
}
