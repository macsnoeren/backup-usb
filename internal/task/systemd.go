package task

import (
	"fmt"
	"regexp"
	"strings"
)

// File names on Linux.
const (
	ServiceTemplate = "usb-backup@.service"
	UdevRule        = "99-usb-backup.rules"
)

// TimerName returns the base name of the timer and its service.
func TimerName(t Trigger) string { return "usb-backup-" + string(t) }

var fsUUIDRe = regexp.MustCompile(`^[A-Za-z0-9-]{1,64}$`)

// checkArgs refuses control characters (a newline would start a new
// directive in the unit file).
func checkArgs(s Spec) error {
	for _, a := range append([]string{s.Command}, s.Args...) {
		for _, r := range a {
			if r < 0x20 || r == 0x7f {
				return fmt.Errorf("control character in %q", a)
			}
		}
	}
	return nil
}

// systemdQuote quotes an argument for ExecStart.
func systemdQuote(a string) string {
	if a != "" && !strings.ContainsAny(a, " \t\"'\\$%;") {
		return a
	}
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "$", "$$", "%", "%%")
	return `"` + r.Replace(a) + `"`
}

func execStart(s Spec) string {
	parts := []string{systemdQuote(s.Command)}
	for _, a := range s.Args {
		parts = append(parts, systemdQuote(a))
	}
	return strings.Join(parts, " ")
}

const hardening = `NoNewPrivileges=yes
PrivateTmp=yes
ProtectControlGroups=yes
ProtectKernelModules=yes
ProtectKernelTunables=yes
RestrictSUIDSGID=yes
Nice=10
IOSchedulingClass=idle
TimeoutStartSec=12h
`

// SystemdFiles returns the files for a spec: name -> content. For the USB
// trigger a service template and a udev rule that starts it for the file
// system UUIDs of the registered disks; for daily/weekly a timer and its
// service.
func SystemdFiles(s Spec) (map[string]string, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	if err := checkArgs(s); err != nil {
		return nil, err
	}
	files := map[string]string{}
	switch s.Trigger {
	case USB:
		if len(s.VolumeUUIDs) == 0 {
			return nil, fmt.Errorf("no registered disks: enroll a disk first")
		}
		files[ServiceTemplate] = "[Unit]\nDescription=USB Backup to the disk with file system UUID %I\n\n[Service]\nType=oneshot\nExecStart=" + execStart(s) + "\n" + hardening
		var rule strings.Builder
		rule.WriteString("# USB Backup: start a backup when a registered disk is plugged in.\n")
		for _, u := range s.VolumeUUIDs {
			if !fsUUIDRe.MatchString(u) {
				return nil, fmt.Errorf("invalid file system UUID %q", u)
			}
			fmt.Fprintf(&rule, "ACTION==\"add\", SUBSYSTEM==\"block\", ENV{ID_FS_UUID}==\"%s\", TAG+=\"systemd\", ENV{SYSTEMD_WANTS}+=\"usb-backup@%s.service\"\n", u, u)
		}
		files[UdevRule] = rule.String()
	case Daily, Weekly:
		name := TimerName(s.Trigger)
		cal := "*-*-* " + s.At + ":00"
		if s.Trigger == Weekly {
			cal = strings.ToUpper(s.Day[:1]) + strings.ToLower(s.Day[1:]) + " " + cal
		}
		files[name+".service"] = "[Unit]\nDescription=USB Backup (" + string(s.Trigger) + ")\n\n[Service]\nType=oneshot\nExecStart=" + execStart(s) + "\n" + hardening
		files[name+".timer"] = "[Unit]\nDescription=USB Backup (" + string(s.Trigger) + ")\n\n[Timer]\nOnCalendar=" + cal + "\nPersistent=true\n\n[Install]\nWantedBy=timers.target\n"
	}
	return files, nil
}
