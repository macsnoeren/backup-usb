package archive

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/macsnoeren/backup-usb/internal/testutil"
)

func TestManifestWithSha256sum(t *testing.T) {
	sum, err := exec.LookPath("sha256sum")
	if err != nil {
		t.Skip("sha256sum not installed")
	}
	src := makeSource(t)
	dest := filepath.Join(t.TempDir(), "Data.zip")
	if _, err := WriteFileTo(t, src, dest, defaultOpts()); err != nil {
		t.Fatal(err)
	}
	out := extract(t, dest+".part")
	cmd := exec.Command(sum, "-c", "--strict", filepath.FromSlash(ManifestName))
	cmd.Dir = out
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("sha256sum -c failed: %v\n%s", err, b)
	}
	// A changed byte is caught.
	if err := os.WriteFile(filepath.Join(out, "a", "b", "c", "deep.txt"), []byte("Deep"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command(sum, "-c", "--quiet", filepath.FromSlash(ManifestName))
	cmd.Dir = out
	if err := cmd.Run(); err == nil {
		t.Fatal("sha256sum accepted a changed file")
	}
}

func TestHashesAreOverFileBytes(t *testing.T) {
	src := t.TempDir()
	content := strings.Repeat("abc", 100000)
	testutil.WriteFile(t, src, "x.txt", content)
	var buf bytes.Buffer
	res, err := Write(context.Background(), src, &buf, defaultOpts())
	if err != nil {
		t.Fatal(err)
	}
	want := sha256.Sum256([]byte(content))
	if res.Entries[0].SHA256 != hex.EncodeToString(want[:]) {
		t.Fatal("sha256 mismatch")
	}
	m, err := ParseManifest(bytes.NewReader(res.Manifest))
	if err != nil || m["x.txt"] != hex.EncodeToString(want[:]) || len(m) != 1 {
		t.Fatalf("manifest %v %v", m, err)
	}
}

func TestManifestEscaping(t *testing.T) {
	sum := strings.Repeat("a", 64)
	for _, name := range []string{"plain.txt", `back\slash`, "new\nline", "sp ace", "  two  spaces"} {
		m, err := ParseManifest(strings.NewReader(ManifestLine(sum, name)))
		if err != nil || m[name] != sum {
			t.Errorf("%q: %v %v", name, m, err)
		}
	}
	for _, bad := range []string{"xyz  a\n", strings.Repeat("g", 64) + "  a\n", sum + " a\n", sum + "  a\n" + sum + "  a\n"} {
		if _, err := ParseManifest(strings.NewReader(bad)); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func FuzzParseManifest(f *testing.F) {
	f.Add([]byte(strings.Repeat("0", 64) + "  file.txt\n"))
	f.Add([]byte(`\` + strings.Repeat("0", 64) + `  a\nb` + "\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		m, err := ParseManifest(bytes.NewReader(data))
		if err != nil {
			return
		}
		// Whatever parses must round-trip.
		var b strings.Builder
		for name, sum := range m {
			b.WriteString(ManifestLine(sum, name))
		}
		back, err := ParseManifest(strings.NewReader(b.String()))
		if err != nil || len(back) != len(m) {
			t.Fatalf("round trip: %v", err)
		}
		for k, v := range m {
			if back[k] != v {
				t.Fatalf("round trip of %q", k)
			}
		}
	})
}
