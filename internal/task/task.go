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

package task

import "fmt"

// Trigger is when the backup starts.
type Trigger string

// USB starts a backup when a registered disk is plugged in.
const USB Trigger = "usb"

// Older versions could also start a backup at a time of day. Those tasks
// can only be removed now.
const (
	OldDaily  Trigger = "daily"
	OldWeekly Trigger = "weekly"
)

// Triggers lists every trigger that can be removed.
var Triggers = []Trigger{USB, OldDaily, OldWeekly}

// Spec describes a task to register.
type Spec struct {
	Trigger Trigger
	// Command is the full path of the program; Args its arguments.
	Command string
	Args    []string
	// VolumeUUIDs are the file system UUIDs of the registered disks (Linux
	// udev rule).
	VolumeUUIDs []string
}

// Registrar registers and removes tasks.
type Registrar interface {
	Install(spec Spec) error
	// Uninstall removes a task; a task that does not exist is fine.
	Uninstall(t Trigger) error
	// Installed reports whether a task exists.
	Installed(t Trigger) bool
}

// Validate checks the spec.
func (s Spec) Validate() error {
	if s.Trigger != USB {
		return fmt.Errorf("unknown trigger %q", s.Trigger)
	}
	if s.Command == "" {
		return fmt.Errorf("no program to start")
	}
	return nil
}

// Name returns the task name for a trigger.
func Name(t Trigger) string {
	switch t {
	case OldDaily:
		return "USB Backup (daily)"
	case OldWeekly:
		return "USB Backup (weekly)"
	}
	return "USB Backup"
}
