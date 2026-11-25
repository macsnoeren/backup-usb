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
	"testing"
	"time"

	"github.com/macsnoeren/backup-usb/internal/app"
	"github.com/macsnoeren/backup-usb/internal/msg"
	"github.com/macsnoeren/backup-usb/internal/secmem"
)

func play(m *Model, lines ...string) View {
	var v View
	for _, l := range lines {
		var e Event
		if err := json.Unmarshal([]byte(l), &e); err != nil {
			panic(err)
		}
		v = m.Apply(e)
	}
	return v
}

func parseEvent(t *testing.T, line string) (e Event) {
	t.Helper()
	if err := json.Unmarshal([]byte(line), &e); err != nil {
		t.Fatal(err)
	}
	return e
}

// TestEveryState plays fixed JSON lines, as a fake CLI would, for every
// state of the status screen.
func TestEveryState(t *testing.T) {
	now := time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	report := func(r app.RunReport) string { b, _ := json.Marshal(r); return string(b) }
	cases := []struct {
		name  string
		lines []string
		state State
		head  string
		btn   Button
		color Color
	}{
		{"no disk", []string{`{"event":"error","code":2,"key":"err.noDisk","args":["BACKUP1"]}`}, StateNoDisk, "Insert USB disk BACKUP1", ButtonNone, Neutral},
		{"period done, other disk", []string{`{"event":"error","code":3,"key":"err.periodDone","args":["2026-W47","BACKUP1","BACKUP2"]}`}, StateWrongDisk, "The backup of 2026-W47 was made on BACKUP1. Next: BACKUP2", ButtonNone, Orange},
		{"period done, no disk", []string{`{"event":"error","code":2,"key":"err.noDiskDone","args":["2026-W47","BACKUP1","BACKUP2"]}`}, StateNoDisk, "The backup of 2026-W47 was made on BACKUP1. Next: BACKUP2", ButtonNone, Neutral},
		{"wrong disk", []string{`{"event":"error","code":3,"key":"err.wrongDisk","args":["2026-W47","BACKUP1","BACKUP2"]}`}, StateWrongDisk, "It is the turn of BACKUP1. Remove BACKUP2 and insert BACKUP1", ButtonNone, Orange},
		{"unknown disk", []string{`{"event":"error","code":6,"key":"err.unknownDisk","args":["STICK"]}`}, StateUnknownDisk, "This disk is not registered", ButtonNone, Orange},
		{"busy", []string{`{"event":"state","state":"searching"}`, `{"event":"state","state":"backup","tag":"Boekhouding","index":2,"count":3}`, `{"event":"progress","tag":"Boekhouding","pct":40}`}, StateBusy, "Backup in progress, do not remove the disk", ButtonCancel, Neutral},
		{"verifying", []string{`{"event":"state","state":"backup","tag":"Data","index":1,"count":1}`, `{"event":"state","state":"verifying","tag":"Data","index":1,"count":1}`}, StateVerifying, "Checking the backup", ButtonNone, Neutral},
		{"done", []string{`{"event":"result","code":0,"report":` + report(app.RunReport{Status: "ok"}) + `}`}, StateDone, "Everything is fine. You can remove the disk", ButtonEject, Green},
		{"ejecting", []string{`{"event":"state","state":"ejecting","disk":"BACKUP1"}`}, StateVerifying, "The backup is fine. Ejecting BACKUP1, do not unplug it yet", ButtonNone, Neutral},
		{"done and ejected", []string{`{"event":"result","code":0,"report":` + report(app.RunReport{Status: "ok", Disk: "BACKUP1", Messages: []string{"x"}, MessageKeys: []string{"msg.ejected"}}) + `}`}, StateDone, "Everything is fine. BACKUP1 was ejected, you can unplug it", ButtonNone, Green},
		{"done, not ejected", []string{`{"event":"result","code":0,"report":` + report(app.RunReport{Status: "ok", Disk: "BACKUP1", Messages: []string{"x"}, MessageKeys: []string{"msg.ejectFailed"}}) + `}`}, StateDone, "The backup is fine, but BACKUP1 is still in use. Click \"Safely remove disk\" before you unplug it", ButtonEject, Orange},
		{"done with warning", []string{`{"event":"result","code":1,"report":` + report(app.RunReport{Status: "warning", Sources: []app.SourceReport{{Tag: "Data", SkippedCount: 3}}}) + `}`}, StateDoneWarning, "Backup made, but 3 files were in use", ButtonDetails, Orange},
		{"failed", []string{`{"event":"error","code":11,"key":"err.write","message":"writing to the backup disk failed"}`}, StateFailed, "Backup failed", ButtonRetry, Red},
		{"needs update", []string{`{"event":"result","code":0,"report":` + report(app.RunReport{Status: "ok", MessageKeys: []string{"msg.needsUpdate"}}) + `}`}, StateNeedsUpdate, "Everything is fine. You can remove the disk", ButtonEject, Green},
	}
	for _, tc := range cases {
		m := NewModel(msg.EN, clock)
		v := play(m, tc.lines...)
		if v.State != tc.state || v.Headline != tc.head || v.Button != tc.btn || v.Color != tc.color {
			t.Errorf("%s: %+v", tc.name, v)
		}
	}
	m := NewModel(msg.EN, clock)
	play(m, `{"event":"state","state":"backup","tag":"Boekhouding","index":2,"count":3}`)
	now = now.Add(time.Minute)
	v := play(m, `{"event":"progress","tag":"Boekhouding","pct":25}`)
	if v.Detail != "Boekhouding, 2 of 3 — 25%, about 3 minutes left" || v.Progress != 25 {
		t.Fatalf("detail %q", v.Detail)
	}
	m = NewModel(msg.NL, clock)
	v = play(m, `{"event":"error","code":3,"key":"err.wrongDisk","args":["2026-W47","BACKUP1","BACKUP2"]}`)
	if v.Headline != "BACKUP1 is aan de beurt. Haal BACKUP2 eruit en plaats BACKUP1" {
		t.Fatal(v.Headline)
	}
}

func TestFooter(t *testing.T) {
	m := NewModel(msg.EN, time.Now)
	last := time.Date(2026, 11, 11, 9, 30, 0, 0, time.Local)
	m.SetStatus(&app.StatusReport{Due: []string{"BACKUP1"}, Disks: []app.DiskStatus{{Name: "BACKUP1", Health: "Healthy", Folders: 12, FreeBytes: 40 << 30, TotalBytes: 160 << 30, LastBackup: &last}}})
	if v := m.View(); v.Footer != "BACKUP1: 12 backups on this disk, 40.0 GiB free of 160.0 GiB (25%)\nLast backup: 2026-11-11 09:30, health: Healthy" {
		t.Fatal(v.Footer)
	}
	// After a backup the numbers come from the report: the disk is ejected.
	rep, _ := json.Marshal(app.RunReport{Status: "ok", Disk: "BACKUP1", Started: time.Date(2026, 11, 18, 10, 0, 0, 0, time.Local), FolderCount: 13, FreeBytes: 32 << 30, TotalBytes: 160 << 30, Health: "Warning"})
	after := play(m, `{"event":"result","code":0,"report":`+string(rep)+`}`)
	if after.Footer != "BACKUP1: 13 backups on this disk, 32.0 GiB free of 160.0 GiB (20%)\nLast backup: 2026-11-18 10:00, health: Warning" {
		t.Fatal(after.Footer)
	}
	// First backup ever, size unknown, window opened without status.
	rep, _ = json.Marshal(app.RunReport{Status: "ok", Disk: "BACKUP2", Started: last, FolderCount: 1, FreeBytes: 1 << 30, Health: "Unknown"})
	nl := NewModel(msg.NL, time.Now)
	if f := play(nl, `{"event":"result","code":0,"report":`+string(rep)+`}`).Footer; f != "BACKUP2: 1 back-up op deze schijf, 1.0 GiB vrij\nLaatste back-up: 2026-11-11 09:30, gezondheid: Unknown" {
		t.Fatal(f)
	}
	v := play(m, `{"event":"error","code":2,"key":"err.noDiskAny"}`)
	if v.Headline != "Insert USB disk BACKUP1" {
		t.Fatal(v.Headline)
	}
}

func TestSessionTimeout(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	pw := secmem.From([]byte("operator password"))
	raw := pw.Bytes()
	s := NewSession(pw, 5*time.Minute, clock)
	now = now.Add(4 * time.Minute)
	if s.Expired() {
		t.Fatal("expired too early")
	}
	if p := s.Password(); string(p) != "operator password" {
		t.Fatal("password")
	}
	now = now.Add(4 * time.Minute) // 4 minutes after the last use
	if s.Expired() {
		t.Fatal("activity did not extend the session")
	}
	now = now.Add(6 * time.Minute)
	if s.Password() != nil {
		t.Fatal("a call after the timeout revived the session")
	}
	if !s.Expired() || s.Password() != nil {
		t.Fatal("session did not end after the timeout")
	}
	for _, c := range raw[:cap(raw)] {
		if c != 0 {
			t.Fatal("password not wiped after the timeout")
		}
	}
}
