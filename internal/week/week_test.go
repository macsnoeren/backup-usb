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
	}{
		{d(2026, 12, 31), ISOWeek, `{jaar}\{week}`, "2026/53", "2026-W53"},
		{d(2027, 1, 1), ISOWeek, `{year}\{week}`, "2026/53", "2026-W53"},
		{d(2027, 1, 4), ISOWeek, `{year}\{week}`, "2027/01", "2027-W01"},
		{d(2026, 1, 29), ISOWeek, `{year}\{week}`, "2026/05", "2026-W05"},
		{d(2026, 1, 29), ISOWeek, `{year}-{week}`, "2026-05", "2026-W05"},
		{d(2026, 11, 22), ISOWeek, `{year}/W{week}`, "2026/W47", "2026-W47"},
		{d(2026, 11, 22), Day, `{jaar}\{maand}\{dag}`, "2026/11/22", "2026-11-22"},
		{d(2026, 3, 5), Month, `{year}\{month}`, "2026/03", "2026-03"},
		{d(2026, 3, 5), Month, DefaultPattern(Month), "2026-03", "2026-03"},
		{d(2026, 3, 5), Day, DefaultPattern(Day), "2026/03-05", "2026-03-05"},
		{d(2026, 3, 5), ISOWeek, DefaultPattern(ISOWeek), "2026/10", "2026-W10"},
		{d(2026, 3, 5).Add(5 * time.Minute), Insert, DefaultPattern(Insert), "2026/03-05_1205", "2026-03-05T12:05"},
		{d(2026, 3, 5), Insert, `{jaar}\{maand}\{dag}\{uur}-{minuut}`, "2026/03/05/12-00", "2026-03-05T12:00"},
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
		if per.Label() != tt.label {
			t.Errorf("%v: label %s", tt.when, per.Label())
		}
		back, ok := p.Match(tt.folder)
		if !ok || back.Label() != tt.label {
			t.Errorf("match %q: %v %v", tt.folder, back.Label(), ok)
		}
		if per.Start.After(tt.when) || !per.End().After(tt.when) {
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

// After a change of schedule a disk holds folders of both: the default
// patterns must never mix them up.
func TestDefaultPatternsDoNotClash(t *testing.T) {
	for _, a := range Schedules() {
		pa, err := ParsePattern(DefaultPattern(a), a)
		if err != nil || pa.Schedule() != a {
			t.Fatalf("%s: %v", a, err)
		}
		for _, b := range Schedules() {
			pb, _ := ParsePattern(DefaultPattern(b), b)
			if pa.Clashes(pb) {
				t.Errorf("%s clashes with %s", a, b)
			}
			for day := 0; day < 400; day += 5 {
				folder := pa.Folder(For(d(2026, 1, 1).AddDate(0, 0, day), a))
				if _, ok := pb.Match(folder); ok != (a == b) {
					t.Fatalf("%s folder %s matches %s: %v", a, folder, b, ok)
				}
			}
		}
	}
}

func TestClashes(t *testing.T) {
	for _, tc := range []struct {
		a  string
		as Schedule
		b  string
		bs Schedule
		ok bool
	}{
		{`{year}\{week}`, ISOWeek, `{year}\{month}`, Month, true},
		{`{year}\{week}`, ISOWeek, `{year}/{month}/{day}`, Day, true}, // inside it
		{`{year}-{month}`, Month, `{year}-{month}\{day}`, Day, true},
		{`{year}\W{week}`, ISOWeek, `{year}\{month}`, Month, false},
		{`{year}\{week}`, ISOWeek, `{year}-{week}`, ISOWeek, false},
		{`{year}\{week}`, ISOWeek, `{jaar}\{week}`, ISOWeek, false},
	} {
		pa, err := ParsePattern(tc.a, tc.as)
		pb, err2 := ParsePattern(tc.b, tc.bs)
		if err != nil || err2 != nil {
			t.Fatal(err, err2)
		}
		if pa.Clashes(pb) != tc.ok || pb.Clashes(pa) != tc.ok {
			t.Errorf("%s and %s: %v", tc.a, tc.b, !tc.ok)
		}
	}
}

func TestClamp(t *testing.T) {
	per := For(d(2026, 11, 25), Month)
	in := d(2026, 11, 3)
	if got := per.Clamp(in); !got.Equal(in) {
		t.Error(got)
	}
	if got := per.Clamp(d(2020, 1, 1)); !got.Equal(per.Start) {
		t.Error(got)
	}
	if got := per.Clamp(d(2030, 1, 1)); !got.Before(per.End()) || got.Before(per.Start) {
		t.Error(got)
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
		{`{year}\{month}-{day}`, Insert},
		{`{year}\{month}-{day}_{minute}{hour}`, Insert},
		{`{year}\{month}-{day}_{hour}{minute}`, Day},
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
		if _, ok := p.Match(rel); ok {
			t.Errorf("%q matched", rel)
		}
	}
	if _, ok := p.Match(`2026\53`); !ok {
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
		{"2026-11-22T09:05", Insert, "2026-11-22T09:05", true},
		{"2026-11-22T24:00", Insert, "", false},
		{"2026-11-31T09:05", Insert, "", false},
		{"2026-11-22", Insert, "", false},
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
	f.Add(`{year}\{month}-{day}_{hour}{minute}`, "2026/03-29_0230")
	f.Fuzz(func(t *testing.T, pattern, rel string) {
		for _, s := range Schedules() {
			p, err := ParsePattern(pattern, s)
			if err != nil {
				continue
			}
			if per, ok := p.Match(rel); ok {
				if p.Folder(per) != rel && p.Folder(per) != normalize(rel) {
					t.Fatalf("pattern %q (%s) matched %q but formats %q", pattern, s, rel, p.Folder(per))
				}
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
