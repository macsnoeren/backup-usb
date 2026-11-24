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
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func TestChoose(t *testing.T) {
	now := For(d(2026, 11, 25), ISOWeek) // 2026-W48
	tests := []struct {
		name  string
		per   Period
		rot   Rotation
		disks []Slot
		want  Turn
	}{
		{"no disks", now, InTurn, nil, Turn{}},
		{"one disk, never used", now, InTurn, []Slot{{Name: "A"}}, Turn{Due: []string{"A"}, Next: []string{"A"}}},
		{"one disk, used last week", now, InTurn, []Slot{{"A", 4, "2026-W47"}}, Turn{Due: []string{"A"}, Next: []string{"A"}}},
		{"nothing known: every disk may start", now, InTurn, []Slot{{Name: "A"}, {Name: "B"}, {Name: "C"}}, Turn{Due: []string{"A", "B", "C"}, Next: []string{"A", "B", "C"}}},
		{"longest ago", now, InTurn, []Slot{{"A", 3, "2026-W47"}, {"B", 1, "2026-W45"}, {"C", 2, "2026-W46"}}, Turn{Due: []string{"B"}, Next: []string{"B"}}},
		{"a new disk goes first", now, InTurn, []Slot{{"A", 3, "2026-W47"}, {"B", 2, "2026-W46"}, {Name: "C"}}, Turn{Due: []string{"C"}, Next: []string{"C"}}},
		{"done this week: refresh only", now, InTurn, []Slot{{"A", 3, "2026-W48"}, {"B", 2, "2026-W47"}}, Turn{Due: []string{"A"}, Done: true, Next: []string{"B"}}},
		{"done, one disk", now, InTurn, []Slot{{"A", 3, "2026-W48"}}, Turn{Due: []string{"A"}, Done: true, Next: []string{"A"}}},
		{"weeks without a backup skip no disk", now, InTurn, []Slot{{"A", 2, "2026-W40"}, {"B", 1, "2026-W39"}}, Turn{Due: []string{"B"}, Next: []string{"B"}}},
		{"a wrong clock does not matter", now, InTurn, []Slot{{"A", 2, "2031-W01"}, {"B", 1, "2026-W46"}}, Turn{Due: []string{"B"}, Next: []string{"B"}}},
		{"any", now, Any, []Slot{{"A", 3, "2026-W48"}, {Name: "B"}}, Turn{Due: []string{"A", "B"}, Next: []string{"A", "B"}}},
		{"insert: never the same disk twice", For(d(2026, 11, 25), Insert), InTurn, []Slot{{"A", 2, For(d(2026, 11, 25), Insert).Label()}, {"B", 1, "2026-11-25T11:00"}}, Turn{Due: []string{"B"}, Next: []string{"B"}}},
	}
	for _, tt := range tests {
		got, err := Choose(tt.per, tt.rot, tt.disks)
		if err != nil || !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s: got %+v, %v; want %+v", tt.name, got, err, tt.want)
		}
	}
	if _, err := Choose(now, "weird", nil); err == nil {
		t.Fatal("unknown rotation accepted")
	}
}

// Every number of disks with every schedule: all disks are used in a fixed
// order, also when periods are skipped.
func TestInTurnCycles(t *testing.T) {
	for _, s := range Schedules() {
		for n := 1; n <= 5; n++ {
			disks := make([]Slot, n)
			for i := range disks {
				disks[i].Name = string(rune('A' + i))
			}
			now, seq := d(2026, 12, 14), uint64(0)
			var used []string
			for i := 0; i < 4*n; i++ {
				per := For(now, s)
				turn, err := Choose(per, InTurn, disks)
				if err != nil || len(turn.Due) == 0 || turn.Done {
					t.Fatalf("%s %d disks: %+v %v", s, n, turn, err)
				}
				used = append(used, turn.Due[0])
				for k := range disks {
					if disks[k].Name == turn.Due[0] {
						seq++
						disks[k].Seq, disks[k].Period = seq, per.Label()
					}
				}
				skip := time.Duration(1 + i%3)
				now = For(now, s).End().Add(skip * per.End().Sub(per.Start))
			}
			for i := n; i < len(used); i++ {
				if used[i] != used[i-n] {
					t.Fatalf("%s %d disks: %v", s, n, used)
				}
			}
		}
	}
}

func TestRotationJSON(t *testing.T) {
	for in, want := range map[string]Rotation{"oddEven": InTurn, "roundRobin": InTurn, "inTurn": InTurn, "any": Any, "x": "x"} {
		var r Rotation
		if err := json.Unmarshal([]byte(`"`+in+`"`), &r); err != nil || r != want {
			t.Errorf("%s: %q %v", in, r, err)
		}
	}
	var r Rotation
	if json.Unmarshal([]byte("3"), &r) == nil {
		t.Error("a number is accepted")
	}
}
