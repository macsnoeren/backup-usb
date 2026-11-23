//go:build integration

package cli

import (
	"strings"
	"testing"
	"time"
)

func TestQuietSkipAndWait(t *testing.T) {
	w := newWorld(t)
	w.enrollUnsigned(map[string]string{"BACKUP1": "odd", "BACKUP2": "even"})
	w.mustRun(0, "run")
	// 4 (recent) and 3 (wrong disk) are silent with --quiet-skip.
	for _, setup := range []func(){func() {}, func() { w.clock.Advance(time.Hour); w.disks.Remove("BACKUP1") }} {
		setup()
		code, out, errOut := w.run("run", "--quiet-skip")
		if (code != 4 && code != 3) || out != "" || errOut != "" {
			t.Fatalf("quiet skip: %d %q %q", code, out, errOut)
		}
	}
	// Real problems are never silent.
	w.set("encryption", "age")
	code, _, errOut := w.run("run", "--quiet-skip", "--any-disk")
	if code != 10 || !strings.Contains(errOut, "encryption is required") {
		t.Fatalf("real error hidden: %d %q", code, errOut)
	}
	// --wait: the CLI waits 0 seconds by default, the trigger passes
	// waitSeconds (30); a disk that shows up in time is used.
	w.set("encryption", "auto")
	w.disks.Remove("BACKUP2")
	start := w.clock.Now()
	w.mustRun(2, "run")
	if w.clock.Now().Sub(start) > time.Second {
		t.Fatal("waited without --wait")
	}
	if code, _, _ := w.run("run", "--wait", "-1"); code != 64 {
		t.Fatal("negative wait accepted")
	}
}
