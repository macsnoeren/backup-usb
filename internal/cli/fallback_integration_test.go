//go:build integration

package cli

import (
	"os"
	"strings"
	"testing"

	"github.com/macsnoeren/backup-usb/internal/testutil"
)

func TestRestoreWithoutIndex(t *testing.T) {
	for _, damage := range []string{"remove", "corrupt"} {
		w := newWorld(t)
		w.enrollUnsigned(map[string]string{"BACKUP1": "odd"})
		bg := w.keysInit()
		w.mustRun(0, "run")
		ix := w.diskFile("BACKUP1", "Backup/2026/47/Data.index.age")
		if damage == "remove" {
			os.Remove(ix)
		} else {
			data, _ := os.ReadFile(ix)
			data[len(data)-10] ^= 1
			os.WriteFile(ix, data, 0o600)
		}
		to := t.TempDir()
		_, errOut := w.mustRun(0, "restore", "--tag", "Data", "--to", to, "--identity", bg)
		if !strings.Contains(errOut, "Index missing or damaged") {
			t.Fatalf("%s: no warning: %q", damage, errOut)
		}
		tree := testutil.Tree(t, to)
		if tree["notes/todo.txt"] != "buy milk" || tree["report.docx"] == "" {
			t.Fatalf("%s: tree %v", damage, tree)
		}
		out, errOut := w.mustRun(0, "list", "--tag", "Data", "--search", "todo", "--identity", bg)
		if !strings.Contains(out, "notes/todo.txt") || !strings.Contains(errOut, "Index missing") {
			t.Fatalf("%s: list %q %q", damage, out, errOut)
		}
	}
}
