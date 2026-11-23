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
		parts = append(parts, m.P.T("gui.footerDisk", d.Name, last, d.Health, app.HumanBytes(d.FreeBytes)))
	}
	return strings.Join(parts, "\n")
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
		case "done":
			return m.view
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
	v := View{Progress: -1, Footer: m.footer()}
	var rep app.RunReport
	_ = json.Unmarshal(e.Report, &rep)
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
