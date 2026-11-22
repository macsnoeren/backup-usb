package cli

import (
	"bytes"
	"testing"
)

func TestRunWithoutProcess(t *testing.T) {
	var out, errOut bytes.Buffer
	code := Run([]string{"usb-backup-cli"}, bytes.NewReader(nil), &out, &errOut)
	if code != 0 {
		t.Fatalf("exit code %d, stderr %q", code, errOut.String())
	}
	if out.Len() == 0 {
		t.Fatal("expected output")
	}
}
