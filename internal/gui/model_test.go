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
	"strings"
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
		{"wrong disk", []string{`{"event":"error","code":3,"key":"err.wrongDisk","args":["2026-W47","BACKUP1","BACKUP2"]}`}, StateWrongDisk, "This week BACKUP1 is due. Remove BACKUP2 and insert BACKUP1", ButtonNone, Orange},
		{"unknown disk", []string{`{"event":"error","code":6,"key":"err.unknownDisk","args":["STICK"]}`}, StateUnknownDisk, "This disk is not registered", ButtonNone, Orange},
		{"busy", []string{`{"event":"state","state":"searching"}`, `{"event":"state","state":"backup","tag":"Boekhouding","index":2,"count":3}`, `{"event":"progress","tag":"Boekhouding","pct":40}`}, StateBusy, "Backup in progress, do not remove the disk", ButtonCancel, Neutral},
		{"verifying", []string{`{"event":"state","state":"backup","tag":"Data","index":1,"count":1}`, `{"event":"state","state":"verifying","tag":"Data","index":1,"count":1}`}, StateVerifying, "Checking the backup", ButtonNone, Neutral},
		{"done", []string{`{"event":"result","code":0,"report":` + report(app.RunReport{Status: "ok"}) + `}`}, StateDone, "Everything is fine. You can remove the disk", ButtonEject, Green},
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
	if v.Headline != "Deze week is BACKUP1 aan de beurt. Haal BACKUP2 eruit en plaats BACKUP1" {
		t.Fatal(v.Headline)
	}
}

func TestFooter(t *testing.T) {
	m := NewModel(msg.EN, time.Now)
	last := time.Date(2026, 11, 11, 9, 30, 0, 0, time.Local)
	m.SetStatus(&app.StatusReport{Due: []string{"BACKUP1"}, Disks: []app.DiskStatus{{Name: "BACKUP1", Health: "Healthy", FreeBytes: 40 << 30, LastBackup: &last}}})
	if v := m.View(); !strings.Contains(v.Footer, "BACKUP1: last backup 2026-11-11 09:30, health Healthy, 40.0 GiB free") {
		t.Fatal(v.Footer)
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
