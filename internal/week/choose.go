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
	"fmt"
	"sort"
)

// Rotation decides which disk is used in which period.
type Rotation string

// Rotations.
const (
	OddEven    Rotation = "oddEven"
	RoundRobin Rotation = "roundRobin"
	Any        Rotation = "any"
)

// Valid reports whether r is a known rotation.
func (r Rotation) Valid() bool { return r == OddEven || r == RoundRobin || r == Any }

// Parity values of a disk slot for odd/even rotation.
const (
	Odd  = "odd"
	Even = "even"
)

// Slot is a registered disk as seen by the rotation.
type Slot struct {
	Name string
	// Weeks is "odd" or "even" for oddEven rotation.
	Weeks string
	// Order is the position in the registration list, for roundRobin.
	Order int
}

// Choose returns the names of the disks that are due in period p. For oddEven
// that is every disk registered for the parity of p.Number(); for
// roundRobin one disk chosen by p.Ordinal(); for any every disk.
func Choose(p Period, rot Rotation, disks []Slot) ([]string, error) {
	switch rot {
	case Any:
		names := make([]string, 0, len(disks))
		for _, d := range disks {
			names = append(names, d.Name)
		}
		return names, nil
	case OddEven:
		want := Even
		if p.Number()%2 == 1 {
			want = Odd
		}
		var names []string
		for _, d := range disks {
			if d.Weeks == want {
				names = append(names, d.Name)
			}
		}
		return names, nil
	case RoundRobin:
		if len(disks) == 0 {
			return nil, nil
		}
		sorted := append([]Slot(nil), disks...)
		sort.SliceStable(sorted, func(i, j int) bool {
			if sorted[i].Order != sorted[j].Order {
				return sorted[i].Order < sorted[j].Order
			}
			return sorted[i].Name < sorted[j].Name
		})
		n := int64(len(sorted))
		i := ((p.Ordinal() % n) + n) % n
		return []string{sorted[i].Name}, nil
	}
	return nil, fmt.Errorf("unknown rotation %q", rot)
}
