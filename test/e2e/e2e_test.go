//go:build e2e

package e2e

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/macsnoeren/backup-usb/internal/testutil"
)

var binary string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "usb-backup-e2e")
	if err != nil {
		panic(err)
	}
	binary = filepath.Join(dir, "usb-backup-cli")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	cmd := exec.Command("go", "build", "-tags", "fakes", "-ldflags", "-X main.version=e2e-test", "-o", binary, "./cmd/usb-backup-cli")
	cmd.Dir = testutil.RepoRoot()
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		panic("build failed: " + err.Error() + "\n" + string(out))
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

type result struct {
	code           int
	stdout, stderr string
}

// cli runs the real binary with extra environment variables and stdin.
func cli(t *testing.T, env []string, stdin string, args ...string) result {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdin = strings.NewReader(stdin)
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	err := cmd.Run()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return result{code, out.String(), errOut.String()}
}

func TestVersionAndHelp(t *testing.T) {
	r := cli(t, nil, "", "version")
	if r.code != 0 || !strings.Contains(r.stdout, "e2e-test") {
		t.Fatalf("version: %+v", r)
	}
	r = cli(t, nil, "", "help")
	if r.code != 0 || !strings.Contains(r.stdout, "Usage") {
		t.Fatalf("help: %+v", r)
	}
	r = cli(t, nil, "", "no-such-command")
	if r.code != 64 {
		t.Fatalf("unknown command: exit %d", r.code)
	}
}
