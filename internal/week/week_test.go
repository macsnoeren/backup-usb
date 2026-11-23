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

package week

import (
	"testing"
	"time"

	_ "github.com/macsnoeren/backup-usb/internal/testutil" // -update flag
)

func d(y int, m time.Month, day int) time.Time { return time.Date(y, m, day, 12, 0, 0, 0, time.Local) }

func TestFolders(t *testing.T) {
	tests := []struct {
		when    time.Time
		s       Schedule
		pattern string
		folder  string
		label   string
		number  int
	}{
		{d(2026, 12, 31), ISOWeek, `{jaar}\{week}`, "2026/53", "2026-W53", 53},
		{d(2027, 1, 1), ISOWeek, `{year}\{week}`, "2026/53", "2026-W53", 53},
		{d(2027, 1, 4), ISOWeek, `{year}\{week}`, "2027/01", "2027-W01", 1},
		{d(2026, 1, 29), ISOWeek, `{year}\{week}`, "2026/05", "2026-W05", 5},
		{d(2026, 1, 29), ISOWeek, `{year}-{week}`, "2026-05", "2026-W05", 5},
		{d(2026, 11, 22), ISOWeek, `{year}/W{week}`, "2026/W47", "2026-W47", 47},
		{d(2026, 11, 22), Day, `{jaar}\{maand}\{dag}`, "2026/11/22", "2026-11-22", 22},
		{d(2026, 3, 5), Month, `{year}\{month}`, "2026/03", "2026-03", 3},
	}
	for _, tt := range tests {
		p, err := ParsePattern(tt.pattern, tt.s)
		if err != nil {
			t.Fatalf("%s: %v", tt.pattern, err)
		}
		per := For(tt.when, tt.s)
		if got := p.Folder(per); got != tt.folder {
			t.Errorf("%v %s: folder %q, want %q", tt.when, tt.pattern, got, tt.folder)
		}
		if per.Label() != tt.label || per.Number() != tt.number {
			t.Errorf("%v: label %s number %d", tt.when, per.Label(), per.Number())
		}
		back, ok := p.Match(tt.folder, tt.s)
		if !ok || back.Label() != tt.label {
			t.Errorf("match %q: %v %v", tt.folder, back.Label(), ok)
		}
		if !per.Start.Before(tt.when) || !per.End().After(tt.when) {
			t.Errorf("%v not within [%v, %v)", tt.when, per.Start, per.End())
		}
	}
}

func TestISOWeekStart(t *testing.T) {
	per := For(d(2027, 1, 1), ISOWeek)
	if per.Start.Weekday() != time.Monday || per.Start.Day() != 28 || per.Start.Month() != 12 {
		t.Fatalf("2026-W53 starts %v", per.Start)
	}
}

func TestOrdinalIsContinuous(t *testing.T) {
	for _, s := range []Schedule{ISOWeek, Day, Month} {
		start := d(2025, 12, 1)
		prev := For(start, s)
		for i := 0; i < 800; i++ {
			next := For(prev.End().Add(time.Hour), s)
			if next.Ordinal() != prev.Ordinal()+1 {
				t.Fatalf("%s: %s -> %s: ordinal %d -> %d", s, prev.Label(), next.Label(), prev.Ordinal(), next.Ordinal())
			}
			prev = next
		}
	}
}

func TestPatternErrors(t *testing.T) {
	bad := []struct {
		p string
		s Schedule
	}{
		{"", ISOWeek},
		{`{year}`, ISOWeek},
		{`{week}`, ISOWeek},
		{`{year}\{week}\{day}`, ISOWeek},
		{`{year}\{month}`, ISOWeek},
		{`{year}{week}`, ISOWeek},
		{`{year}\\{week}`, ISOWeek},
		{`\{year}\{week}`, ISOWeek},
		{`{year}\..\{week}`, ISOWeek},
		{`{year}\{wek}`, ISOWeek},
		{`{year}\{week`, ISOWeek},
		{`{year}\{week}\{week}`, ISOWeek},
		{`{year} {week}`, ISOWeek},
		{`{year}\{month}`, Day},
		{`{year}\{month}`, "yearly"},
	}
	for _, b := range bad {
		if _, err := ParsePattern(b.p, b.s); err == nil {
			t.Errorf("pattern %q (%s) accepted", b.p, b.s)
		}
	}
}

func TestMatchIsStrict(t *testing.T) {
	p, _ := ParsePattern(`{year}\{week}`, ISOWeek)
	for _, rel := range []string{"2026/5", "2026/054", "2026/00", "2026/54", "2027/53", "foto's/01", "2026", "2026/01/x", "../2026/01", "1969/01", " 2026/01"} {
		if _, ok := p.Match(rel, ISOWeek); ok {
			t.Errorf("%q matched", rel)
		}
	}
	if _, ok := p.Match(`2026\53`, ISOWeek); !ok {
		t.Error("2026-W53 exists")
	}
}

func TestParseLabel(t *testing.T) {
	for _, tc := range []struct {
		in   string
		s    Schedule
		want string
		ok   bool
	}{
		{"2026-47", ISOWeek, "2026-W47", true},
		{"2026-W05", ISOWeek, "2026-W05", true},
		{"2027-53", ISOWeek, "", false},
		{"2026-11-22", Day, "2026-11-22", true},
		{"2026-02-30", Day, "", false},
		{"2026-11", Month, "2026-11", true},
		{"x", Month, "", false},
	} {
		got, err := ParseLabel(tc.in, tc.s)
		if (err == nil) != tc.ok || (tc.ok && got.Label() != tc.want) {
			t.Errorf("%s: %v %v", tc.in, got.Label(), err)
		}
	}
}

func FuzzPattern(f *testing.F) {
	f.Add(`{year}\{week}`, "2026/47")
	f.Add(`{jaar}-{week}`, "2026-01")
	f.Fuzz(func(t *testing.T, pattern, rel string) {
		p, err := ParsePattern(pattern, ISOWeek)
		if err != nil {
			return
		}
		if per, ok := p.Match(rel, ISOWeek); ok {
			if p.Folder(per) != rel && p.Folder(per) != normalize(rel) {
				t.Fatalf("pattern %q matched %q but formats %q", pattern, rel, p.Folder(per))
			}
		}
	})
}

func normalize(s string) string {
	out := []rune(s)
	for i, r := range out {
		if r == '\\' {
			out[i] = '/'
		}
	}
	return string(out)
}
