package task

import (
	"fmt"
	"regexp"
	"strings"
)

// Trigger is when the backup starts.
type Trigger string

// Triggers.
const (
	USB    Trigger = "usb"
	Daily  Trigger = "daily"
	Weekly Trigger = "weekly"
)

// Triggers lists all triggers.
var Triggers = []Trigger{USB, Daily, Weekly}

// Spec describes a task to register.
type Spec struct {
	Trigger Trigger
	// At is the time of day, "02:00", for daily and weekly.
	At string
	// Day is the weekday for weekly: mon, tue, wed, thu, fri, sat, sun.
	Day string
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
	Uninstall(t Trigger) error
}

var timeRe = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)

var days = map[string]string{"mon": "Monday", "tue": "Tuesday", "wed": "Wednesday", "thu": "Thursday", "fri": "Friday", "sat": "Saturday", "sun": "Sunday"}

// Validate checks the spec.
func (s Spec) Validate() error {
	switch s.Trigger {
	case USB:
	case Daily:
		if !timeRe.MatchString(s.At) {
			return fmt.Errorf("--at %q is not a time like 02:00", s.At)
		}
	case Weekly:
		if !timeRe.MatchString(s.At) {
			return fmt.Errorf("--at %q is not a time like 18:00", s.At)
		}
		if _, ok := days[strings.ToLower(s.Day)]; !ok {
			return fmt.Errorf("--day %q is not one of mon, tue, wed, thu, fri, sat, sun", s.Day)
		}
	default:
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
	case Daily:
		return "USB Backup (daily)"
	case Weekly:
		return "USB Backup (weekly)"
	}
	return "USB Backup"
}
