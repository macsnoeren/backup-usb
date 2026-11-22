//go:build integration

package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/macsnoeren/backup-usb/internal/app"
	"github.com/macsnoeren/backup-usb/internal/testutil"
)

func TestList(t *testing.T) {
	w := newWorld(t)
	w.enrollUnsigned(map[string]string{"BACKUP1": "odd"})
	testutil.WriteFile(t, w.srcDir, "Data/projects/2026/alpha/plan.txt", "plan")
	testutil.WriteFile(t, w.srcDir, "Data/projects/2026/beta/Plan B.txt", "plan b")
	bg := w.keysInit()
	w.mustRun(0, "run")

	out, _ := w.mustRun(0, "list")
	if !strings.Contains(out, "2026-W47") || !strings.Contains(out, "Books, Data") || !strings.Contains(out, "encrypted") {
		t.Fatalf("periods:\n%s", out)
	}
	// Files of an encrypted backup need a key.
	w.mustRun(64, "list", "--tag", "Data", "--identity", "/nonexistent")
	out, _ = w.mustRun(0, "list", "--tag", "Data", "--week", "2026-47", "--identity", bg)
	if !strings.Contains(out, "projects/") || !strings.Contains(out, "report.docx") || strings.Contains(out, "alpha") {
		t.Fatalf("top level:\n%s", out)
	}
	out, _ = w.mustRun(0, "list", "--tag", "Data", "--path", "projects/2026", "--identity", bg)
	if !strings.Contains(out, "projects/2026/alpha/") || !strings.Contains(out, "projects/2026/beta/") || strings.Contains(out, "plan.txt") {
		t.Fatalf("nested:\n%s", out)
	}
	out, _ = w.mustRun(0, "list", "--tag", "Data", "--search", "PLAN", "--json", "--identity", bg)
	var res app.ListResult
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatal(err)
	}
	if len(res.Entries) != 2 || !res.FromIndex || res.Entries[0].Path != "projects/2026/alpha/plan.txt" {
		t.Fatalf("search: %+v", res)
	}
	// With the vault password on stdin.
	w.stdin = testPassword + "\n"
	w.mustRun(0, "list", "--tag", "Books", "--password-stdin", "--vault", w.vaultPath())
}
