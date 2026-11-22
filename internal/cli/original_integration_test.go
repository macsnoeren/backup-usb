//go:build integration

package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/macsnoeren/backup-usb/internal/app"
)

func TestRestoreOriginalNeverOverwrites(t *testing.T) {
	w := newWorld(t)
	w.enrollUnsigned(map[string]string{"BACKUP1": "odd"})
	w.mustRun(0, "run")
	todo := filepath.Join(w.srcDir, "Data", "notes", "todo.txt")
	os.WriteFile(todo, []byte("changed since the backup"), 0o644)
	os.Remove(filepath.Join(w.srcDir, "Data", "report.docx"))
	out, _ := w.mustRun(0, "restore", "--tag", "Data", "--original", "--json")
	var rep app.RestoreReport
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(todo); string(got) != "changed since the backup" {
		t.Fatal("existing file overwritten")
	}
	restored := filepath.Join(w.srcDir, "Data", "notes", "todo (restored 2026-11-18).txt")
	if got, _ := os.ReadFile(restored); string(got) != "buy milk" {
		t.Fatal("renamed copy missing")
	}
	if _, err := os.Stat(filepath.Join(w.srcDir, "Data", "report.docx")); err != nil {
		t.Fatal("missing file not restored to its place")
	}
	// Every file that still exists is restored under a new name (never
	// overwritten); report.docx was gone and is back under its own name.
	for _, r := range rep.Renamed {
		if r.Path == "report.docx" {
			t.Fatal("report.docx renamed although its place was free")
		}
	}
	// A second restore picks the next free name.
	w.mustRun(0, "restore", "--tag", "Data", "--original", "--file", "notes/todo.txt")
	if _, err := os.Stat(filepath.Join(w.srcDir, "Data", "notes", "todo (restored 2026-11-18 2).txt")); err != nil {
		t.Fatal("second copy name")
	}
}
