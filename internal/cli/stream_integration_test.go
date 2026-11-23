// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Maurice Snoeren
//
// This file is part of USB Backup.
//
// USB Backup is free software: you can redistribute it and/or modify it under
// the terms of the GNU General Public License as published by the Free Software
// Foundation, either version 3 of the License, or (at your option) any later
// version.
//
// USB Backup is distributed in the hope that it will be useful, but WITHOUT ANY
// WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A
// PARTICULAR PURPOSE. See the GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License along with
// USB Backup. If not, see <https://www.gnu.org/licenses/>.

//go:build integration

package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/macsnoeren/backup-usb/internal/app"
	"github.com/macsnoeren/backup-usb/internal/testutil"
)

func streamLines(t *testing.T, out string) []map[string]any {
	var lines []map[string]any
	sc := bufio.NewScanner(strings.NewReader(out))
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for sc.Scan() {
		var m map[string]any
		if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
			t.Fatalf("not a JSON line: %q", sc.Text())
		}
		lines = append(lines, m)
	}
	return lines
}

func TestJSONStreamRun(t *testing.T) {
	w := newWorld(t)
	w.enrollUnsigned(map[string]string{"BACKUP1": "odd"})
	code, out, _ := w.run("run", "--json-stream")
	if code != 0 {
		t.Fatal(code, out)
	}
	lines := streamLines(t, out)
	events := map[string]int{}
	for _, l := range lines {
		events[l["event"].(string)]++
	}
	if events["state"] < 2 || events["progress"] < 1 || events["result"] != 1 {
		t.Fatalf("events %v", events)
	}
	last := lines[len(lines)-1]
	if last["event"] != "result" || last["code"].(float64) != 0 {
		t.Fatalf("last %v", last)
	}
	var rep app.RunReport
	data, _ := json.Marshal(last["report"])
	if err := json.Unmarshal(data, &rep); err != nil || rep.Disk != "BACKUP1" {
		t.Fatalf("report %v %v", rep, err)
	}
	// An error ends with an error line and the exit code.
	w.disks.Remove("BACKUP1")
	code, out, _ = w.run("run", "--json-stream")
	lines = streamLines(t, out)
	last = lines[len(lines)-1]
	if code != 2 || last["event"] != "error" || last["code"].(float64) != 2 || last["key"] != "err.noDisk" {
		t.Fatalf("error line %v", last)
	}
}

// cancelWriter sends "cancel" on stdin when it sees the first progress line.
type cancelWriter struct {
	mu   sync.Mutex
	buf  bytes.Buffer
	pw   *io.PipeWriter
	sent bool
}

func (c *cancelWriter) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.buf.Write(p)
	if !c.sent && bytes.Contains(p, []byte(`"event":"progress"`)) {
		c.sent = true
		go c.pw.Write([]byte("{\"cancel\":true}\n"))
	}
	return len(p), nil
}

func TestJSONStreamCancel(t *testing.T) {
	w := newWorld(t)
	w.enrollUnsigned(map[string]string{"BACKUP1": "odd"})
	for i := 0; i < 400; i++ {
		testutil.WriteFile(t, w.srcDir, fmt.Sprintf("Data/many/f%04d.txt", i), strings.Repeat("x", 20000))
	}
	pr, pw := io.Pipe()
	env := w.env()
	env.Stdin = pr
	cw := &cancelWriter{pw: pw}
	env.Stdout, env.Stderr = cw, io.Discard
	code := env.Run([]string{"usb-backup-cli", "run", "--json-stream"})
	pw.Close()
	if code != 11 {
		t.Fatalf("exit %d\n%s", code, cw.buf.String())
	}
	tree := testutil.Tree(t, w.disks.Disk("BACKUP1").Mount)
	for name := range tree {
		if strings.HasSuffix(name, ".part") {
			t.Fatalf("part file left after cancel: %s", name)
		}
	}
	if !strings.Contains(cw.buf.String(), `"key":"err.cancelled"`) {
		t.Fatalf("no cancelled error: %s", cw.buf.String())
	}
}

func TestJSONStreamPassword(t *testing.T) {
	w := newWorld(t)
	w.enrollUnsigned(map[string]string{"BACKUP1": "odd"})
	w.keysInit()
	w.mustRun(0, "run")
	// The password is the first line on stdin, then cancel lines may follow.
	w.stdin = testPassword + "\n"
	code, out, errOut := w.run("list", "--tag", "Data", "--json-stream", "--password-stdin", "--vault", w.vaultPath())
	if code != 0 {
		t.Fatalf("%d %s %s", code, out, errOut)
	}
	if !strings.Contains(out, `"path":"notes","dir":true`) {
		t.Fatalf("%s", out)
	}
	for _, s := range []string{out, errOut} {
		if strings.Contains(s, testPassword) {
			t.Fatal("password in the output")
		}
	}
	w.stdin = "not the password\n"
	code, out, _ = w.run("verify", "--json-stream", "--vault", w.vaultPath(), "--password-stdin")
	if code != 13 || strings.Contains(out, "not the password") || !strings.Contains(out, "err.vaultPassword") {
		t.Fatalf("%d %s", code, out)
	}
}

func TestJSONStreamArgsAndEject(t *testing.T) {
	w := newWorld(t)
	w.enrollUnsigned(map[string]string{"BACKUP1": "odd", "BACKUP2": "even"})
	w.disks.Remove("BACKUP1")
	_, out, _ := w.run("run", "--json-stream")
	lines := streamLines(t, out)
	last := lines[len(lines)-1]
	args, _ := last["args"].([]any)
	if last["key"] != "err.wrongDisk" || len(args) != 3 || args[1] != "BACKUP1" || args[2] != "BACKUP2" {
		t.Fatalf("%v", last)
	}
	w.mustRun(0, "run", "--any-disk")
	_, out, _ = w.run("run", "--any-disk", "--force", "--json-stream")
	if !strings.Contains(out, `"state":"verifying"`) {
		t.Fatal("no verifying state")
	}
	w.mustRun(0, "eject")
	if code, _, _ := w.run("status", "--json"); code != 0 {
		t.Fatal("status after eject")
	}
	w.mustRun(2, "eject")
}
