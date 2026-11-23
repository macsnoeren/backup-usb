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

package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"

	"github.com/macsnoeren/backup-usb/internal/app"
)

// streamCommands support --json; with --json-stream their report goes into
// the result event.
var streamCommands = map[string]bool{
	"run": true, "verify": true, "list": true, "restore": true, "status": true,
	"disks": true, "enroll": true, "version": true, "eject": true,
}

// streamer writes JSON lines to the real stdout.
type streamer struct {
	mu  sync.Mutex
	out io.Writer
}

func (s *streamer) write(v any) {
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, _ = s.out.Write(append(data, '\n'))
}

// StreamResult is the last line of a --json-stream run.
type StreamResult struct {
	Event   string          `json:"event"` // result or error
	Code    int             `json:"code"`
	Key     string          `json:"key,omitempty"`
	Args    []any           `json:"args,omitempty"`
	Message string          `json:"message,omitempty"`
	Report  json.RawMessage `json:"report,omitempty"`
	Output  string          `json:"output,omitempty"`
}

// runStream runs a command in --json-stream mode: events as JSON lines on
// stdout while it runs, then one result or error line. A line "cancel" or
// {"cancel":true} on stdin cancels it.
func (e *Env) runStream(c *ctx, cmd *command, args []string) int {
	st := &streamer{out: e.Stdout}
	c.stream = st
	var buf bytes.Buffer
	inner := *e
	inner.Stdout = &buf
	c.env = &inner
	if streamCommands[cmd.name] {
		args = append(args, "--json")
	}
	err := cmd.run(c, args)
	code := ExitCode(err)
	res := StreamResult{Event: "result", Code: code}
	if out := bytes.TrimSpace(buf.Bytes()); len(out) > 0 {
		if json.Valid(out) {
			res.Report = out
		} else {
			res.Output = string(out)
		}
	}
	if err != nil && code != ExitWarning {
		res.Event = "error"
		var ae *app.Error
		if errors.As(err, &ae) {
			res.Key, res.Args = ae.Key, ae.Args
		}
		res.Message = appErrText(c, err)
		if res.Message == "" {
			res.Message = err.Error()
		}
	}
	st.write(res)
	return code
}

// watchCancel cancels ctx when the GUI sends a cancel line on stdin. It
// starts only after any password line has been read.
func (c *ctx) watchCancel(cancel context.CancelFunc) {
	if c.stream == nil || c.env.Stdin == nil {
		return
	}
	go func() {
		sc := bufio.NewScanner(c.env.Stdin)
		sc.Buffer(make([]byte, 1024), 4096)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line == "cancel" || strings.Contains(strings.ReplaceAll(line, " ", ""), `"cancel":true`) {
				cancel()
				return
			}
		}
	}()
}

// emitter returns the event callback for app.Deps in stream mode.
func (c *ctx) emitter() func(app.Event) {
	if c.stream == nil {
		return nil
	}
	return func(ev app.Event) { c.stream.write(ev) }
}
