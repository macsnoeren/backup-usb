//go:build integration

package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/macsnoeren/backup-usb/internal/testutil"
)

// stableSources gives every source file and folder fixed times and modes,
// so the zips and their hashes are the same on every run.
func stableSources(t *testing.T, dir string) {
	stamp := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	err := filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		mode := os.FileMode(0o644)
		if info.IsDir() {
			mode = 0o755
		}
		if err := os.Chmod(p, mode); err != nil {
			return err
		}
		return os.Chtimes(p, stamp, stamp)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestReportGoldenFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file attributes in the zip differ on Windows, so the hashes do too")
	}
	w := newWorld(t)
	w.clock.Set(time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC))
	w.enrollUnsigned(map[string]string{"BACKUP1": "odd", "BACKUP2": "even"})
	stableSources(t, w.srcDir)
	out, _ := w.mustRun(0, "run", "--json")
	mount := w.disks.Disk("BACKUP1").Mount
	norm := strings.NewReplacer(mount, "<mount>", strings.ReplaceAll(mount, `\`, `\\`), "<mount>")
	testutil.Golden(t, "run-week47.json", []byte(norm.Replace(out)))
	report, err := os.ReadFile(w.diskFile("BACKUP1", "Backup/2026/47/report.txt"))
	if err != nil {
		t.Fatal(err)
	}
	testutil.Golden(t, "run-week47-report.txt", report)
}
