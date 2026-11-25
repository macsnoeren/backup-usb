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
	"reflect"
	"testing"
	"time"

	"github.com/macsnoeren/backup-usb/internal/msg"
	"github.com/macsnoeren/backup-usb/internal/week"
)

func TestParseOptions(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want Options
	}{
		{[]string{"x"}, Options{}},
		{nil, Options{}},
		{[]string{"x", "--auto"}, Options{Auto: true}},
		{[]string{"x", "--tray"}, Options{Tray: true}},
		{[]string{"x", "--settings"}, Options{Settings: true}},
		{[]string{"x", "--other"}, Options{}},
	} {
		if got := ParseOptions(tc.args); got != tc.want {
			t.Errorf("%v: %+v", tc.args, got)
		}
	}
}

func TestSecondStart(t *testing.T) {
	for _, tc := range []struct {
		o    Options
		want SecondAction
	}{
		{Options{}, SecondShow},
		{Options{Auto: true}, SecondCheck},
		{Options{Tray: true}, SecondNothing},
	} {
		if got := SecondStart(tc.o); got != tc.want {
			t.Errorf("%+v: %v", tc.o, got)
		}
	}
}

// A hidden window comes forward for a registered disk and for problems, and
// stays away otherwise.
func TestAttention(t *testing.T) {
	const (
		backup  = `{"event":"state","state":"backup","tag":"Data","index":1,"count":2,"disk":"BACKUP1"}`
		done    = `{"event":"result","code":0,"report":{"status":"ok","disk":"BACKUP1"}}`
		wrong   = `{"event":"error","code":3,"key":"err.wrongDisk","args":["2026-W47","BACKUP1","BACKUP2"]}`
		recent  = `{"event":"error","code":4,"key":"err.recent","args":[20]}`
		warning = `{"event":"result","code":1,"report":{"status":"warning","disk":"BACKUP1","sources":[{"tag":"Data","skippedCount":2}]}}`
	)
	for _, tc := range []struct {
		name     string
		line     string
		inserted bool
		want     Attention
	}{
		{"searching", `{"event":"state","state":"searching"}`, true, AttentionNone},
		{"backup", backup, true, AttentionShow},
		{"backup at login", backup, false, AttentionShow},
		{"done", done, true, AttentionNotify},
		{"warning", warning, true, AttentionShow},
		{"still in use", `{"event":"result","code":0,"report":{"status":"ok","disk":"BACKUP1","messages":["x"],"messageKeys":["msg.ejectFailed"]}}`, true, AttentionShow},
		{"wrong disk", wrong, true, AttentionShow},
		{"wrong disk at login", wrong, false, AttentionNone},
		{"period done", `{"event":"error","code":3,"key":"err.periodDone","args":["W","A","B"]}`, true, AttentionShow},
		{"recent", recent, true, AttentionShow},
		{"recent at login", recent, false, AttentionNone},
		{"no disk", `{"event":"error","code":2,"key":"err.noDisk","args":["BACKUP1"]}`, true, AttentionNone},
		{"none registered", `{"event":"error","code":2,"key":"err.noDisksRegistered"}`, true, AttentionNone},
		{"unknown disk", `{"event":"error","code":6,"key":"err.unknownDisk","args":["STICK"]}`, true, AttentionNone},
		{"other backup", `{"event":"error","code":5,"key":"err.busy"}`, true, AttentionNone},
		{"cancelled", `{"event":"error","code":11,"key":"err.cancelled"}`, true, AttentionNone},
		{"failed", `{"event":"error","code":11,"key":"err.write","message":"x"}`, true, AttentionShow},
		{"changed settings at login", `{"event":"error","code":7,"key":"err.tampered","message":"x"}`, false, AttentionShow},
		{"crash", `{"event":"error","code":-1,"message":"stopped"}`, false, AttentionShow},
	} {
		m := NewModel(msg.EN, time.Now)
		e := parseEvent(t, tc.line)
		a := Attendant{Inserted: tc.inserted}
		if got := a.Next(e, m.Apply(e), false); got != tc.want {
			t.Errorf("%s: %v", tc.name, got)
		}
		if got := (&Attendant{Inserted: tc.inserted}).Next(e, m.View(), true); got != AttentionNone {
			t.Errorf("%s, visible: %v", tc.name, got)
		}
	}
	// Once per run: who hides it during the backup is left alone, also when
	// the window was visible at the start.
	for _, visible := range []bool{false, true} {
		m := NewModel(msg.EN, time.Now)
		a := Attendant{Inserted: true}
		e := parseEvent(t, backup)
		a.Next(e, m.Apply(e), visible)
		for _, l := range []string{backup, `{"event":"state","state":"verifying","tag":"Data","index":1,"count":2}`, `{"event":"progress","tag":"Data","pct":5}`} {
			e := parseEvent(t, l)
			if got := a.Next(e, m.Apply(e), false); got != AttentionNone {
				t.Errorf("again for %s: %v", l, got)
			}
		}
		e = parseEvent(t, warning)
		if got := a.Next(e, m.Apply(e), false); got != AttentionShow {
			t.Errorf("warning at the end: %v", got)
		}
	}
}

func TestBusyIsNoFailure(t *testing.T) {
	m := NewModel(msg.EN, time.Now)
	v := play(m, `{"event":"error","code":5,"key":"err.busy","message":"x"}`)
	if v.State != StateOtherBackup || v.Color != Orange || v.Button != ButtonRetry || v.Headline != "Another backup is running" {
		t.Fatalf("%+v", v)
	}
}

func TestArrivals(t *testing.T) {
	var a Arrivals
	check := func(step string, present map[string]string, want ...string) {
		t.Helper()
		if got, ok := a.Update(present, a.Gen()); !ok || !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: %v, want %v", step, got, want)
		}
	}
	one := map[string]string{"BACKUP1": `E:\`}
	two := map[string]string{"BACKUP1": `E:\`, "BACKUP2": `F:\`}
	check("nothing", nil)
	check("plugged in", one, "BACKUP1")
	check("another stick", one)
	check("second disk", two, "BACKUP2")
	check("second disk out", one)
	check("second disk back", two, "BACKUP2")
	// Out and in again before anyone looked.
	a.Removed(DriveLetters(1 << ('E' - 'A')))
	check("first disk again", two, "BACKUP1")
	a.Removed(DriveLetters(1 << ('Z' - 'A')))
	check("other letter out", two)
	check("other letter for the same disk", map[string]string{"BACKUP1": `G:\`, "BACKUP2": `F:\`})
	a.Removed("g")
	check("lower case letter", map[string]string{"BACKUP1": `G:\`, "BACKUP2": `F:\`}, "BACKUP1")
	// Two at once, in a fixed order.
	check("all out", nil)
	check("two at once", two, "BACKUP1", "BACKUP2")
	// A backup that could not start yet is offered again.
	a.Forget("BACKUP2")
	check("again", two, "BACKUP2")
	// A disk the run itself found is not new any more.
	check("all out again", nil)
	a.Seen("BACKUP1")
	check("seen by the run", one)
	a.Seen("")
	a.Removed("")
	check("still", one)
	// After a run: an ejected disk is gone, one that came meanwhile is new.
	a.Refresh(map[string]string{"BACKUP2": `F:\`})
	check("after eject", two, "BACKUP1", "BACKUP2")
	a.Refresh(one)
	check("second one back", two, "BACKUP2")
	// An answer from before a removal is not used: the disk would be known
	// when it is plugged in again.
	gen := a.Gen()
	a.Removed("E")
	if fresh, ok := a.Update(two, gen); ok || fresh != nil {
		t.Fatalf("old answer used: %v", fresh)
	}
	check("plugged in again", two, "BACKUP1")
}

func TestTarget(t *testing.T) {
	c := Connected{Mounts: map[string]string{"A": `E:\`, "B": `F:\`, "C": `G:\`, "D": ""}, Due: map[string]bool{"A": true, "C": true, "D": true}}
	for _, tc := range []struct {
		fresh  []string
		target string
		rest   []string
	}{
		{[]string{"C"}, "C", nil}, // not A, which is also there and due
		{[]string{"B"}, "", nil},
		{[]string{"D"}, "", nil},
		{[]string{"A", "B", "C"}, "A", []string{"B", "C"}},
		{[]string{"B", "C"}, "C", []string{"B"}},
	} {
		if target, rest := Target(tc.fresh, c); target != tc.target || len(rest) != len(tc.rest) || (len(rest) > 0 && !reflect.DeepEqual(rest, tc.rest)) {
			t.Errorf("%v: %q %v", tc.fresh, target, rest)
		}
	}
}

func TestFollowUp(t *testing.T) {
	busy, cancelled, other := Event{Key: "err.busy"}, Event{Key: "err.cancelled"}, Event{}
	for _, tc := range []struct {
		last           Event
		state          State
		pending, fresh bool
		want           Follow
	}{
		{other, StateDone, false, true, FollowNone},
		{other, StateDone, true, true, FollowAll},
		{other, StateWrongDisk, true, false, FollowAll},
		{other, StateDoneWarning, true, true, FollowDue},
		{other, StateFailed, true, true, FollowDue},
		{other, StateFailed, false, true, FollowNone},
		{cancelled, StateFailed, true, true, FollowNone},
		// Never at once: the other program can be busy for hours.
		{busy, StateOtherBackup, true, true, FollowLater},
		{busy, StateOtherBackup, true, false, FollowLater},
		{busy, StateOtherBackup, false, true, FollowLater},
		{busy, StateOtherBackup, false, false, FollowNone},
	} {
		if got := FollowUp(tc.last, View{State: tc.state}, tc.pending, tc.fresh); got != tc.want {
			t.Errorf("%+v: %v", tc, got)
		}
	}
}

func TestWrongDisk(t *testing.T) {
	c := Connected{Mounts: map[string]string{"A": `E:\`, "B": `F:\`, "C": `G:\`}, Due: map[string]bool{"A": true, "C": true}}
	e, ok := WrongDisk([]string{"B"}, c)
	m := NewModel(msg.EN, time.Now)
	v := m.Apply(e)
	if !ok || v.State != StateWrongDisk || v.Headline != "It is the turn of A / C. Remove B and insert A / C" {
		t.Fatalf("%v %+v", ok, v)
	}
	if _, ok := WrongDisk([]string{"B"}, Connected{Mounts: map[string]string{"B": `F:\`}, Due: map[string]bool{}}); ok {
		t.Fatal("no other disk: the run explains it")
	}
}

func TestDriveLetters(t *testing.T) {
	for mask, want := range map[uint32]string{0: "", 1: "A", 1 << 4: "E", 1<<4 | 1<<5: "EF", 1 << 25: "Z", 1 << 26: "", 0xFFFFFFFF: "ABCDEFGHIJKLMNOPQRSTUVWXYZ"} {
		if got := DriveLetters(mask); got != want {
			t.Errorf("%x: %q", mask, got)
		}
	}
}

func TestStartWhenReady(t *testing.T) {
	for _, s := range week.Schedules() {
		if !StartWhenReady(s, false) {
			t.Errorf("%s: opening the window must start a backup", s)
		}
		if got := StartWhenReady(s, true); got != (s != week.Insert) {
			t.Errorf("%s passive: %v", s, got)
		}
	}
}

func TestStale(t *testing.T) {
	for _, s := range []State{StateNoDisk, StateWrongDisk, StateDone, StateSkipped, StateOtherBackup} {
		if Stale(View{State: s}, time.Minute) || !Stale(View{State: s}, time.Hour) {
			t.Error(s)
		}
	}
	// Problems stay until someone saw them; a backup is never disturbed.
	for _, s := range []State{StateFailed, StateDoneWarning, StateNeedsUpdate, StateBusy, StateVerifying} {
		if Stale(View{State: s}, 100*time.Hour) {
			t.Error(s)
		}
	}
	// Not ejected: the message still matters.
	if Stale(View{State: StateDone, Color: Orange}, time.Hour) {
		t.Error("orange")
	}
}

func TestTrayTip(t *testing.T) {
	p := msg.Printer{Lang: msg.EN}
	if got := TrayTip(p, View{Headline: "Backup in progress", Progress: 42}); got != "USB Backup: Backup in progress (42%)" {
		t.Fatal(got)
	}
	if got := TrayTip(p, View{Headline: "Insert USB disk BACKUP1", Progress: -1}); got != "USB Backup: Insert USB disk BACKUP1" {
		t.Fatal(got)
	}
	if got := TrayTip(p, View{Progress: -1}); got != "USB Backup" {
		t.Fatal(got)
	}
}

func TestTraySettings(t *testing.T) {
	ctx := context.Background()
	on, err := fakeCLI(t, "stream", `FAKE_LINES={"event":"result","code":0,"report":{"trayIcon":true,"schedule":"insert"}}`).TraySettings(ctx)
	if err != nil || !on.Icon || on.Schedule != week.Insert {
		t.Fatalf("%+v %v", on, err)
	}
	off, err := fakeCLI(t, "stream", `FAKE_LINES={"event":"result","code":0,"report":{}}`).TraySettings(ctx)
	if err != nil || off.Icon || off.Schedule != week.ISOWeek {
		t.Fatalf("%+v %v", off, err)
	}
	if bad, err := fakeCLI(t, "stream", `FAKE_LINES={"event":"error","code":7,"message":"changed"}`).TraySettings(ctx); err == nil || bad.Icon {
		t.Fatalf("%+v", bad)
	}
}

func TestConnectedDisks(t *testing.T) {
	ctx := context.Background()
	got, err := fakeCLI(t, "stream", `FAKE_LINES={"event":"result","code":0,"report":{"registered":[{"name":"BACKUP1","connected":true,"mount":"E:\\","due":true},{"name":"BACKUP2","connected":false}],"connected":[{"mount":"F:\\","state":"unknown"}]}}`).ConnectedDisks(ctx)
	if err != nil || !reflect.DeepEqual(got, Connected{Mounts: map[string]string{"BACKUP1": `E:\`}, Due: map[string]bool{"BACKUP1": true}}) {
		t.Fatalf("%v %v", got, err)
	}
	for _, l := range []string{`{"event":"error","code":7,"message":"changed"}`, `{"event":"result","code":0,"report":[]}`} {
		if _, err := fakeCLI(t, "stream", "FAKE_LINES="+l).ConnectedDisks(ctx); err == nil {
			t.Errorf("%s accepted", l)
		}
	}
}

func TestStartsAtLogin(t *testing.T) {
	for line, want := range map[string]bool{
		`{"event":"result","code":0,"report":{"usb":false,"login":true}}`: true,
		`{"event":"result","code":0,"report":{"usb":true}}`:               false,
		`{"event":"error","code":11,"message":"x"}`:                       false,
	} {
		if got := fakeCLI(t, "stream", "FAKE_LINES="+line).StartsAtLogin(context.Background()); got != want {
			t.Errorf("%s: %v", line, got)
		}
	}
}
