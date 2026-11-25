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

package gui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/macsnoeren/backup-usb/internal/app"
	"github.com/macsnoeren/backup-usb/internal/msg"
	"github.com/macsnoeren/backup-usb/internal/week"
)

// The decisions about the icon next to the clock: which disk is new, when a
// hidden window comes forward. The window itself only carries them out.

// Options are the arguments of the window.
type Options struct {
	// Auto: started by the USB trigger.
	Auto bool
	// Settings: only show the settings, as administrator.
	Settings bool
	// Tray: start hidden, behind the icon (at login).
	Tray bool
}

// ParseOptions reads the arguments; unknown ones are ignored.
func ParseOptions(args []string) (o Options) {
	for i, a := range args {
		if i == 0 {
			continue
		}
		o.Auto = o.Auto || a == "--auto"
		o.Settings = o.Settings || a == settingsArg
		o.Tray = o.Tray || a == "--tray"
	}
	return o
}

const settingsArg = "--settings"

// SecondAction is what a second start does with the window that is there.
type SecondAction int

// Second start actions.
const (
	SecondNothing SecondAction = iota
	// SecondShow brings the window to the front.
	SecondShow
	// SecondCheck tells the window that a disk may be there; only the window
	// knows whether it runs behind the icon.
	SecondCheck
)

// SecondStart decides what a second start does. Windows starts the window
// for every stick, so the USB trigger does not bring it forward by itself.
func SecondStart(o Options) SecondAction {
	switch {
	case o.Tray:
		return SecondNothing
	case o.Auto:
		return SecondCheck
	default:
		return SecondShow
	}
}

// Attention is what a hidden window does about an event.
type Attention int

// Attention values.
const (
	AttentionNone Attention = iota
	// AttentionNotify shows a message at the icon.
	AttentionNotify
	// AttentionShow brings the window forward.
	AttentionShow
)

// Attendant decides for one run whether a hidden window comes forward.
type Attendant struct {
	// Inserted: the run is there because a registered disk was plugged in.
	Inserted bool
	raised   bool
}

// Next decides about an event and the view it gave. A visible window needs
// no attention.
func (a *Attendant) Next(e Event, v View, visible bool) Attention {
	att := AttentionNone
	switch e.Event {
	case "state":
		if v.State == StateBusy || v.State == StateVerifying {
			if !a.raised {
				att = AttentionShow
			}
			a.raised = true // who hides it again is left alone
		}
	case "result":
		att = AttentionShow
		if v.Color == Green {
			att = AttentionNotify
		}
	case "error":
		switch v.State {
		case StateNoDisk, StateUnknownDisk, StateOtherBackup:
		case StateWrongDisk, StateSkipped:
			if a.Inserted {
				att = AttentionShow
			}
		default:
			if e.Key != "err.cancelled" {
				att = AttentionShow
			}
		}
	}
	if visible {
		return AttentionNone
	}
	return att
}

// Arrivals remembers which registered disks are connected, so that only a
// disk that was not there yet starts a backup. Safe for concurrent use.
type Arrivals struct {
	mu    sync.Mutex
	known map[string]string // name -> mount point
	// gen counts removals: an answer from before one says nothing.
	gen uint64
}

// Gen is for Update: ask it before looking at the disks.
func (a *Arrivals) Gen() uint64 {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.gen
}

// Update takes the connected registered disks (name -> mount point) and
// returns the names that were not there before, sorted. ok is false when a
// disk was removed since gen: then look again.
func (a *Arrivals) Update(present map[string]string, gen uint64) (fresh []string, ok bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if gen != a.gen {
		return nil, false
	}
	known := make(map[string]string, len(present))
	for name, mount := range present {
		if _, ok := a.known[name]; !ok {
			fresh = append(fresh, name)
		}
		known[name] = mount
	}
	a.known = known
	sort.Strings(fresh)
	return fresh, true
}

// Refresh takes the connected disks after a run: what is gone is forgotten
// and what is new stays new, because its own event is still to come.
func (a *Arrivals) Refresh(present map[string]string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for name := range a.known {
		if mount, ok := present[name]; ok {
			a.known[name] = mount
		} else {
			delete(a.known, name)
		}
	}
}

// Seen notes a disk that a run found itself.
func (a *Arrivals) Seen(name string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, ok := a.known[name]; ok || name == "" {
		return
	}
	if a.known == nil {
		a.known = map[string]string{}
	}
	a.known[name] = ""
}

// Forget makes disks new again.
func (a *Arrivals) Forget(names ...string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, n := range names {
		delete(a.known, n)
	}
}

// Removed notes that drives (a list of letters) are gone, so a disk that is
// plugged in again before anyone looked is new.
func (a *Arrivals) Removed(letters string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if letters != "" {
		a.gen++
	}
	for name, mount := range a.known {
		if mount != "" && strings.Contains(strings.ToUpper(letters), strings.ToUpper(mount[:1])) {
			delete(a.known, name)
		}
	}
}

// DriveLetters are the letters in the unit mask of a Windows volume event.
func DriveLetters(mask uint32) string {
	var b strings.Builder
	for i := range 26 {
		if mask&(1<<i) != 0 {
			b.WriteByte(byte('A' + i))
		}
	}
	return b.String()
}

// StartWhenReady reports whether a backup starts when the disk that is due
// is already there. passive: the user did not start the program, but logged
// in or clicked the icon. That is not plugging in: with a backup for every
// insert it starts nothing.
func StartWhenReady(s week.Schedule, passive bool) bool { return !passive || s != week.Insert }

// Target chooses the disk for a run from the new disks. A disk that is due
// is named, so a run never takes another one that happens to be there; rest
// are the other new disks, for later. Without one that is due the run itself
// decides ("" and no rest): naming a disk would take it out of turn.
func Target(fresh []string, c Connected) (target string, rest []string) {
	for i, name := range fresh {
		if c.Due[name] && c.Mounts[name] != "" {
			rest = append(append([]string(nil), fresh[:i]...), fresh[i+1:]...)
			return name, rest
		}
	}
	return "", nil
}

// WrongDisk is the message for a new disk that is not due while another one
// that is due is connected. A run would take that other one.
func WrongDisk(fresh []string, c Connected) (Event, bool) {
	var due []string
	for name := range c.Mounts {
		if c.Due[name] {
			due = append(due, name)
		}
	}
	if len(due) == 0 || len(fresh) == 0 {
		return Event{}, false
	}
	sort.Strings(due)
	return Event{Event: "error", Code: 3, Key: "err.wrongDisk", Args: []any{"", strings.Join(due, " / "), fresh[0]}}, true
}

// Follow is what happens after a run with the disks that came meanwhile.
type Follow int

// Follow values.
const (
	// FollowNone: they are noted as known.
	FollowNone Follow = iota
	// FollowLater: after a minute, because another program is busy.
	FollowLater
	// FollowDue: only one that is due gets a run.
	FollowDue
	// FollowAll: as if they were plugged in now.
	FollowAll
)

// FollowUp decides after a run that ended with last and this view. A backup
// that was cancelled does not start again, and a problem is not replaced by
// the message that another disk is not needed. pending: disks came
// meanwhile; fresh: the run was for new disks.
func FollowUp(last Event, v View, pending, fresh bool) Follow {
	switch {
	case last.Key == "err.busy" && (pending || fresh):
		return FollowLater
	case !pending || last.Key == "err.cancelled":
		return FollowNone
	case v.State == StateDoneWarning || v.State == StateFailed:
		return FollowDue
	default:
		return FollowAll
	}
}

// staleAfter is when an old message says nothing about now any more.
const staleAfter = 10 * time.Minute

// Stale reports whether a view of this age must be made again when the
// window is opened. Problems stay until someone saw them.
func Stale(v View, age time.Duration) bool {
	if age < staleAfter {
		return false
	}
	switch v.State {
	case StateNoDisk, StateWrongDisk, StateUnknownDisk, StateSkipped, StateOtherBackup:
		return true
	case StateDone:
		return v.Color != Orange
	}
	return false
}

// TrayTip is the text at the icon.
func TrayTip(p msg.Printer, v View) string {
	tip := p.T("gui.title")
	if v.Headline != "" {
		tip += ": " + v.Headline
	}
	if v.Progress >= 0 {
		tip += fmt.Sprintf(" (%d%%)", v.Progress)
	}
	return tip
}

// TraySettings are the settings the icon needs.
type TraySettings struct {
	Icon     bool
	Schedule week.Schedule
}

// TraySettings reads them; on an error the icon is off.
func (c CLI) TraySettings(ctx context.Context) (TraySettings, error) {
	cfg, err := c.Config(ctx)
	if err != nil {
		return TraySettings{}, err
	}
	return TraySettings{Icon: cfg.TrayIcon, Schedule: cfg.Schedule}, nil
}

// Connected are the registered disks that are connected.
type Connected struct {
	Mounts map[string]string // name -> mount point
	Due    map[string]bool   // can be used now
}

// disksTimeout: a disk that is failing must not block every later look.
const disksTimeout = time.Minute

// ConnectedDisks asks which registered disks are connected.
func (c CLI) ConnectedDisks(ctx context.Context) (Connected, error) {
	ctx, cancel := context.WithTimeout(ctx, disksTimeout)
	defer cancel()
	ev := c.Call(ctx, nil, "disks")
	if ev.Event != "result" {
		return Connected{}, errors.New(ev.Message)
	}
	var rep app.DisksReport
	if err := json.Unmarshal(ev.Report, &rep); err != nil {
		return Connected{}, fmt.Errorf("unexpected answer from usb-backup-cli: %w", err)
	}
	out := Connected{Mounts: map[string]string{}, Due: map[string]bool{}}
	for _, d := range rep.Registered {
		if d.Connected {
			out.Mounts[d.Name] = d.Mount
			out.Due[d.Name] = d.Due
		}
	}
	return out, nil
}
