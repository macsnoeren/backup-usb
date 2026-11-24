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
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/macsnoeren/backup-usb/internal/app"
	"github.com/macsnoeren/backup-usb/internal/volume"
)

// ejectProbe records what this process still had open on the disk when it
// was ejected: with anything open, Windows and Linux refuse.
type ejectProbe struct {
	volume.Finder
	calls int
	open  []string
	err   error
}

func (p *ejectProbe) Eject(ctx context.Context, v volume.Volume) error {
	p.calls++
	fds, _ := os.ReadDir("/proc/self/fd")
	for _, fd := range fds {
		if l, err := os.Readlink(filepath.Join("/proc/self/fd", fd.Name())); err == nil && strings.HasPrefix(l, v.Mount) {
			p.open = append(p.open, l)
		}
	}
	if p.err != nil {
		return p.err
	}
	return p.Finder.Eject(ctx, v)
}

func (w *world) runEject(probe *ejectProbe, code int, args ...string) (app.RunReport, string) {
	w.t.Helper()
	env := w.env()
	probe.Finder = env.Finder
	env.Finder = probe
	got, out, errOut := run(w.t, env, append([]string{"run", "--json"}, args...)...)
	if got != code {
		w.t.Fatalf("exit %d, want %d: %s %s", got, code, out, errOut)
	}
	var rep app.RunReport
	_ = json.Unmarshal([]byte(out), &rep)
	return rep, out
}

func hasKey(r app.RunReport, key string) bool {
	for _, k := range r.MessageKeys {
		if k == key {
			return true
		}
	}
	return false
}

func TestEjectAfterBackup(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("looks at /proc/self/fd")
	}
	newRun := func(eject bool) *world {
		w := newWorld(t)
		w.enrollUnsigned(map[string]string{"BACKUP1": "odd", "BACKUP2": "even"})
		w.cfgData["ejectAfter"] = eject
		w.writeConfig()
		return w
	}

	w := newRun(true)
	p := &ejectProbe{}
	rep, _ := w.runEject(p, 0)
	if p.calls != 1 || len(p.open) > 0 {
		t.Fatalf("eject calls %d, still open on the disk: %v", p.calls, p.open)
	}
	if !hasKey(rep, "msg.ejected") || hasKey(rep, "msg.canRemove") || hasKey(rep, "msg.ejectFailed") {
		t.Fatalf("messages %v", rep.MessageKeys)
	}

	// A disk that cannot be ejected: the backup is still good, but nobody
	// may be told that the disk can be pulled out.
	w = newRun(true)
	p = &ejectProbe{err: errors.New("in use")}
	rep, _ = w.runEject(p, 0)
	if rep.Status != app.StatusOK || !hasKey(rep, "msg.ejectFailed") || hasKey(rep, "msg.canRemove") || hasKey(rep, "msg.ejected") {
		t.Fatalf("status %s, messages %v", rep.Status, rep.MessageKeys)
	}

	// Not after a dry run, not when switched off.
	p = &ejectProbe{}
	newRun(true).runEject(p, 0, "--dry-run")
	newRun(false).runEject(p, 0)
	if p.calls != 0 {
		t.Fatalf("ejected %d times", p.calls)
	}

	// Not after a failed backup.
	w = newRun(true)
	if err := os.RemoveAll(filepath.Join(w.srcDir, "Data")); err != nil {
		t.Fatal(err)
	}
	p = &ejectProbe{}
	env := w.env()
	p.Finder, env.Finder = env.Finder, p
	if code, _, _ := run(t, env, "run"); code == 0 || code == 1 || p.calls != 0 {
		t.Fatalf("exit %d, ejected %d times", code, p.calls)
	}
}

// The texts of unregistered disks take no name.
func TestDisksTextHasNoFormatErrors(t *testing.T) {
	w := newWorld(t)
	out, _ := w.mustRun(0, "disks")
	if strings.Contains(out, "%!") || !strings.Contains(out, "not a backup disk") {
		t.Fatal(out)
	}
}
