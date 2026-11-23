//go:build integration

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigSourcesAndImportExport(t *testing.T) {
	a := newWorld(t)
	a.keysInit()
	newSrc := t.TempDir()
	a.stdin = testPassword + "\n"
	a.mustRun(0, "config", "add-source", "--tag", "Photos", "--path", newSrc, "--exclude", "*.raw", "--password-stdin", "--vault", a.vaultPath())
	a.stdin = testPassword + "\n"
	a.mustRun(64, "config", "add-source", "--tag", "photos", "--path", newSrc, "--password-stdin", "--vault", a.vaultPath())
	a.stdin = testPassword + "\n"
	a.mustRun(0, "config", "remove-source", "Books", "--password-stdin", "--vault", a.vaultPath())
	a.stdin = testPassword + "\n"
	a.mustRun(0, "config", "set", "keepPerDisk", "8", "--password-stdin", "--vault", a.vaultPath())
	exported := filepath.Join(t.TempDir(), "settings.json")
	a.mustRun(0, "config", "export", exported)
	data, _ := os.ReadFile(exported)
	for _, secret := range []string{"AGE-SECRET-KEY", testPassword, "signing"} {
		if strings.Contains(string(data), secret) {
			t.Fatalf("export contains %q", secret)
		}
	}
	want, _ := a.mustRun(0, "config", "show")

	// Another machine with its own keys: import, signed with its key.
	b := newWorld(t)
	b.keysInit()
	b.stdin = testPassword + "\n"
	b.mustRun(0, "config", "import", exported, "--password-stdin", "--vault", b.vaultPath())
	got, _ := b.mustRun(0, "config", "show")
	if got != want {
		t.Fatalf("settings differ:\n%s\n---\n%s", got, want)
	}
	b.mustRun(2, "run", "--dry-run") // 2 (no disk), not 7: the signature is valid on b
	// A broken import file is refused and changes nothing.
	os.WriteFile(exported, []byte(`{"keepPerDisk": -1}`), 0o644)
	b.stdin = testPassword + "\n"
	b.mustRun(64, "config", "import", exported, "--password-stdin", "--vault", b.vaultPath())
	if again, _ := b.mustRun(0, "config", "show"); again != want {
		t.Fatal("failed import changed the settings")
	}
}
