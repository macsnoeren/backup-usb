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
	"fmt"
)

// Rotation decides which disk is used next.
type Rotation string

// Rotations.
const (
	// InTurn uses every disk in turn: the one that was used longest ago.
	InTurn Rotation = "inTurn"
	// Any accepts every registered disk.
	Any Rotation = "any"
)

// Valid reports whether r is a known rotation.
func (r Rotation) Valid() bool { return r == InTurn || r == Any }

// UnmarshalJSON reads a rotation. The calendar rotations of older settings
// files (odd and even weeks, round robin) become InTurn.
func (r *Rotation) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("rotation: %w", err)
	}
	if s == "oddEven" || s == "roundRobin" {
		s = string(InTurn)
	}
	*r = Rotation(s)
	return nil
}

// Slot is a registered disk as seen by the rotation.
type Slot struct {
	Name string
	// Seq orders the last backups of the disks, higher is later; 0 means
	// that no backup on this disk is known.
	Seq uint64
	// Period is the label of the period of the last backup.
	Period string
}

// Turn is the outcome of Choose.
type Turn struct {
	// Due are the disks that may be used now.
	Due []string
	// Done is set when Due already has the backup of this period: using it
	// again only refreshes that backup.
	Done bool
	// Next are the disks whose turn it is after this period (when Done), or
	// now.
	Next []string
}

// Choose returns the disks that are due in period p. For Any that is every
// disk. For InTurn a disk that already has the backup of this period stays
// due until the period is over; otherwise it is the disk that was used
// longest ago, where disks that were never used come first. The calendar
// plays no part in the order, so a period without a backup skips no disk.
func Choose(p Period, rot Rotation, disks []Slot) (Turn, error) {
	switch rot {
	case Any:
		names := pick(disks, func(Slot) bool { return true })
		return Turn{Due: names, Next: names}, nil
	case InTurn:
	default:
		return Turn{}, fmt.Errorf("unknown rotation %q", rot)
	}
	var lowest uint64
	for i, d := range disks {
		if i == 0 || d.Seq < lowest {
			lowest = d.Seq
		}
	}
	next := pick(disks, func(d Slot) bool { return d.Seq == lowest })
	if p.Schedule != Insert {
		if done := pick(disks, func(d Slot) bool { return d.Seq > 0 && d.Period == p.Label() }); len(done) > 0 {
			return Turn{Due: done, Done: true, Next: next}, nil
		}
	}
	return Turn{Due: next, Next: next}, nil
}

// pick returns the names of the disks for which ok is true, in their order.
func pick(disks []Slot, ok func(Slot) bool) []string {
	var names []string
	for _, d := range disks {
		if ok(d) {
			names = append(names, d.Name)
		}
	}
	return names
}
