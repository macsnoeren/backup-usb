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
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/macsnoeren/backup-usb/internal/app"
	"github.com/macsnoeren/backup-usb/internal/msg"
)

// State is what the status screen shows.
type State string

// States of the status screen (table "Statusscherm" in the design).
const (
	StateChecking    State = "checking"
	StateNoDisk      State = "noDisk"
	StateWrongDisk   State = "wrongDisk"
	StateUnknownDisk State = "unknownDisk"
	StateBusy        State = "busy"
	StateVerifying   State = "verifying"
	StateDone        State = "done"
	StateDoneWarning State = "doneWarning"
	StateFailed      State = "failed"
	StateNeedsUpdate State = "needsUpdate"
	StateSkipped     State = "skipped"
)

// Button is the one action button.
type Button string

// Buttons.
const (
	ButtonNone    Button = ""
	ButtonCancel  Button = "cancel"
	ButtonEject   Button = "eject"
	ButtonDetails Button = "details"
	ButtonRetry   Button = "retry"
)

// Color is the color of the icon and headline.
type Color string

// Colors.
const (
	Neutral Color = "neutral"
	Green   Color = "green"
	Orange  Color = "orange"
	Red     Color = "red"
)

// View is everything the window draws.
type View struct {
	State    State
	Color    Color
	Headline string
	Detail   string
	Footer   string
	// Progress is 0..100, or -1 for no progress bar.
	Progress int
	Button   Button
	// Details is the text behind the Details button.
	Details string
}

// Model turns CLI events into views.
type Model struct {
	P      msg.Printer
	Now    func() time.Time
	view   View
	due    string
	status *app.StatusReport
	// progress bookkeeping for the remaining time
	tag     string
	started time.Time
}

// NewModel returns a model in the checking state.
func NewModel(lang msg.Lang, now func() time.Time) *Model {
	m := &Model{P: msg.Printer{Lang: lang}, Now: now}
	m.view = View{State: StateChecking, Color: Neutral, Headline: m.P.T("gui.checking"), Progress: -1}
	return m
}

// View returns the current view.
func (m *Model) View() View { return m.view }

// SetStatus takes the output of "status --json" for the footer and the
// disk that is due.
func (m *Model) SetStatus(st *app.StatusReport) {
	m.status = st
	if len(st.Due) > 0 {
		m.due = strings.Join(st.Due, " / ")
	}
	m.view.Footer = m.footer()
}

func (m *Model) footer() string {
	if m.status == nil {
		return ""
	}
	var parts []string
	for _, d := range m.status.Disks {
		last := m.P.T("status.never")
		if d.LastBackup != nil {
			last = d.LastBackup.Local().Format("2006-01-02 15:04")
		}
		free := m.P.T("gui.free", app.HumanBytes(d.FreeBytes))
		if d.TotalBytes > 0 {
			free = m.P.T("gui.freeOf", app.HumanBytes(d.FreeBytes), app.HumanBytes(d.TotalBytes), int(float64(d.FreeBytes)/float64(d.TotalBytes)*100))
		}
		if d.Folders == 1 {
			parts = append(parts, m.P.T("gui.footerOne", d.Name, free, last, d.Health))
		} else {
			parts = append(parts, m.P.T("gui.footerDisk", d.Name, d.Folders, free, last, d.Health))
		}
	}
	return strings.Join(parts, "\n")
}

// noteBackup puts a finished backup in the footer: the disk may be ejected
// by now, so it cannot be asked any more.
func (m *Model) noteBackup(rep *app.RunReport) {
	if rep.Disk == "" || rep.Started.IsZero() {
		return
	}
	st := app.StatusReport{}
	if m.status != nil {
		st = *m.status
	}
	d := app.DiskStatus{Name: rep.Disk}
	var disks []app.DiskStatus
	for _, x := range st.Disks {
		if strings.EqualFold(x.Name, rep.Disk) {
			d = x
		} else {
			disks = append(disks, x)
		}
	}
	started := rep.Started
	d.Folders, d.FreeBytes, d.TotalBytes, d.LastBackup = rep.FolderCount, rep.FreeBytes, rep.TotalBytes, &started
	if rep.Health != "" {
		d.Health = rep.Health
	}
	st.Disks = append([]app.DiskStatus{d}, disks...)
	m.status = &st
}

// Apply processes one event from "run --json-stream" and returns the new
// view.
func (m *Model) Apply(e Event) View {
	v := View{Progress: -1, Footer: m.footer()}
	switch e.Event {
	case "state":
		switch e.State {
		case "searching":
			v.State, v.Color, v.Headline = StateChecking, Neutral, m.P.T("gui.checking")
		case "backup":
			v.State, v.Color, v.Button = StateBusy, Neutral, ButtonCancel
			v.Headline = m.P.T("gui.busy")
			v.Detail = m.P.T("gui.busySource", e.Tag, e.Index, e.Count)
			v.Progress = 0
			m.tag, m.started = e.Tag, m.Now()
		case "verifying":
			v.State, v.Color, v.Headline = StateVerifying, Neutral, m.P.T("gui.verifying")
			v.Detail = m.P.T("gui.busySource", e.Tag, e.Index, e.Count)
			v.Progress = 100
		case "ejecting":
			v.State, v.Color, v.Headline = StateVerifying, Neutral, m.P.T("gui.ejecting", e.Disk)
		default:
			return m.view
		}
	case "progress":
		if m.view.State != StateBusy {
			return m.view
		}
		v = m.view
		v.Progress = e.Pct
		v.Detail = m.busyDetail(e)
	case "result":
		v = m.result(e)
	case "error":
		v = m.failure(e)
	default:
		return m.view
	}
	m.view = v
	return v
}

// busyDetail is "Data, 2 of 3 — 42%, about 3 minutes left".
func (m *Model) busyDetail(e Event) string {
	base := m.view.Detail
	if i := strings.Index(base, " — "); i >= 0 {
		base = base[:i]
	}
	left := ""
	if e.Pct > 3 && e.Pct < 100 {
		elapsed := m.Now().Sub(m.started)
		rest := time.Duration(float64(elapsed) * float64(100-e.Pct) / float64(e.Pct))
		left = m.P.T("gui.timeLeft", humanDuration(m.P, rest))
	}
	return base + " — " + fmt.Sprintf("%d%%", e.Pct) + left
}

func humanDuration(p msg.Printer, d time.Duration) string {
	switch {
	case d < time.Minute:
		return p.T("gui.lessThanMinute")
	case d < time.Hour:
		return p.T("gui.minutes", int(d.Minutes()+0.5))
	}
	return p.T("gui.hours", int(d.Hours()), int(d.Minutes())%60)
}

func (m *Model) result(e Event) View {
	var rep app.RunReport
	_ = json.Unmarshal(e.Report, &rep)
	m.noteBackup(&rep)
	v := View{Progress: -1, Footer: m.footer()}
	needsUpdate := false
	for _, k := range rep.MessageKeys {
		if k == "msg.needsUpdate" {
			needsUpdate = true
		}
	}
	skipped, warnings := 0, 0
	var lines []string
	for _, s := range rep.Sources {
		skipped += s.SkippedCount
		warnings += s.WarningCount
		for _, sk := range s.Skipped {
			lines = append(lines, s.Tag+": "+sk.Path+" ("+sk.Reason+")")
		}
	}
	v.Details = strings.Join(append(lines, rep.Messages...), "\n")
	switch {
	case e.Code == 1 || rep.Status == app.StatusWarning:
		v.State, v.Color, v.Button = StateDoneWarning, Orange, ButtonDetails
		if skipped > 0 {
			v.Headline = m.P.T("gui.doneSkipped", skipped)
		} else {
			v.Headline = m.P.T("gui.doneWarning")
		}
	default:
		v.State, v.Color, v.Button = StateDone, Green, ButtonEject
		v.Headline = m.P.T("gui.done")
	}
	for i, k := range rep.MessageKeys {
		switch {
		case k == "msg.ejected" && v.State == StateDone:
			v.Headline, v.Button = m.P.T("gui.doneEjected", rep.Disk), ButtonNone
		case k == "msg.ejectFailed" && v.State == StateDone:
			v.Headline, v.Color = m.P.T("gui.doneNotEjected", rep.Disk), Orange
		case (k == "msg.ejected" || k == "msg.ejectFailed") && i < len(rep.Messages):
			v.Detail = rep.Messages[i]
		}
	}
	if needsUpdate {
		v.Detail = m.P.T("gui.needsUpdate")
		if v.State == StateDone {
			v.State = StateNeedsUpdate
		}
	}
	return v
}

func argString(args []any, i int) string {
	if i < len(args) {
		return fmt.Sprint(args[i])
	}
	return "?"
}

func (m *Model) failure(e Event) View {
	v := View{Progress: -1, Footer: m.footer()}
	switch e.Key {
	case "err.noDisk", "err.noDiskAny":
		v.State, v.Color = StateNoDisk, Neutral
		due := m.due
		if e.Key == "err.noDisk" {
			due = argString(e.Args, 0)
		}
		v.Headline = m.P.T("gui.insert", due)
	case "err.noDisksRegistered":
		v.State, v.Color, v.Headline = StateNoDisk, Neutral, m.P.T("gui.noDisksRegistered")
	case "err.wrongDisk":
		v.State, v.Color = StateWrongDisk, Orange
		v.Headline = m.P.T("gui.wrongDisk", argString(e.Args, 1), argString(e.Args, 2))
	case "err.unknownDisk":
		v.State, v.Color, v.Headline = StateUnknownDisk, Orange, m.P.T("gui.unknownDisk")
	case "err.duplicate", "err.recent":
		v.State, v.Color, v.Headline, v.Button = StateSkipped, Green, m.P.T("gui.recent"), ButtonEject
	default:
		v.State, v.Color, v.Button = StateFailed, Red, ButtonRetry
		v.Headline = m.P.T("gui.failed")
		v.Detail = e.Message
		if e.Key == "err.cancelled" {
			v.Detail = m.P.T("gui.cancelled")
		}
	}
	return v
}
