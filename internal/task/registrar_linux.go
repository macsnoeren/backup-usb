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

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/macsnoeren/backup-usb/internal/fsx"
)

// Systemd registers systemd units and udev rules.
type Systemd struct {
	// UnitDir and RulesDir default to /etc/systemd/system and
	// /etc/udev/rules.d.
	UnitDir, RulesDir string
	// Run runs a program; nil uses the real one.
	Run func(name string, args ...string) error
}

// NewPlatformRegistrar returns the registrar for this platform.
func NewPlatformRegistrar() Registrar { return &Systemd{} }

var binDirs = []string{"/usr/bin", "/bin", "/usr/sbin", "/sbin"}

func tool(name string) string {
	for _, d := range binDirs {
		p := filepath.Join(d, name)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return name
}

func (s *Systemd) dirs() (string, string) {
	u, r := s.UnitDir, s.RulesDir
	if u == "" {
		u = "/etc/systemd/system"
	}
	if r == "" {
		r = "/etc/udev/rules.d"
	}
	return u, r
}

func (s *Systemd) run(name string, args ...string) error {
	if s.Run != nil {
		return s.Run(name, args...)
	}
	cmd := exec.Command(tool(name), args...) //nolint:gosec // fixed program from system folders, arguments as a list
	cmd.Env = []string{"LC_ALL=C", "PATH=/usr/sbin:/usr/bin:/sbin:/bin"}
	done := make(chan error, 1)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		return nil
	case <-time.After(2 * time.Minute):
		_ = cmd.Process.Kill()
		return fmt.Errorf("%s: timed out", name)
	}
}

// ErrNeedRoot is returned when installing system units without root.
var ErrNeedRoot = errors.New("installing systemd units and udev rules needs root (sudo)")

// Install writes the files and enables them.
func (s *Systemd) Install(spec Spec) error {
	files, err := SystemdFiles(spec)
	if err != nil {
		return err
	}
	if s.UnitDir == "" && os.Geteuid() != 0 {
		return ErrNeedRoot
	}
	if s.UnitDir == "" {
		// The unit runs as root: the program and the configuration folder
		// must not be changeable by other users.
		paths := []string{spec.Command}
		for i, a := range spec.Args {
			if a == "--config" && i+1 < len(spec.Args) {
				paths = append(paths, spec.Args[i+1])
			}
		}
		for _, p := range paths {
			if err := rootOwned(p); err != nil {
				return err
			}
		}
	}
	unitDir, rulesDir := s.dirs()
	for name, content := range files {
		dir := unitDir
		if name == UdevRule {
			dir = rulesDir
		}
		if err := os.MkdirAll(dir, 0o755); err != nil { //nolint:gosec // system unit folders are world-readable
			return err
		}
		if err := fsx.WriteFileAtomic(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			return err
		}
	}
	if err := s.run("systemctl", "daemon-reload"); err != nil {
		return err
	}
	return s.run("udevadm", "control", "--reload")
}

// Installed reports whether the udev rule or the timer exists.
func (s *Systemd) Installed(t Trigger) bool {
	if t == Login {
		return false // only Windows has a window
	}
	unitDir, rulesDir := s.dirs()
	p := filepath.Join(rulesDir, UdevRule)
	if t != USB {
		p = filepath.Join(unitDir, TimerName(t)+".timer")
	}
	_, err := os.Lstat(p)
	return err == nil
}

// rootOwned checks that p and every parent folder belong to root and are
// not writable by group or others.
func rootOwned(p string) error {
	abs, err := filepath.Abs(p)
	if err != nil {
		return err
	}
	for cur := abs; ; cur = filepath.Dir(cur) {
		st, err := os.Stat(cur)
		if err != nil {
			return err
		}
		sys, ok := st.Sys().(*syscall.Stat_t)
		if !ok || sys.Uid != 0 || st.Mode().Perm()&0o022 != 0 && st.Mode()&os.ModeSticky == 0 {
			return fmt.Errorf("%s must belong to root and not be writable by others, because the scheduled task runs as root", cur)
		}
		if cur == filepath.Dir(cur) {
			return nil
		}
	}
}

// Uninstall disables and removes the files of a trigger.
func (s *Systemd) Uninstall(t Trigger) error {
	if t == Login {
		return nil // never there
	}
	if s.UnitDir == "" && os.Geteuid() != 0 {
		return ErrNeedRoot
	}
	unitDir, rulesDir := s.dirs()
	var paths []string
	if t == USB {
		paths = []string{filepath.Join(rulesDir, UdevRule), filepath.Join(unitDir, ServiceTemplate)}
	} else {
		if s.Installed(t) {
			_ = s.run("systemctl", "disable", "--now", TimerName(t)+".timer")
		}
		paths = []string{filepath.Join(unitDir, TimerName(t)+".timer"), filepath.Join(unitDir, TimerName(t)+".service")}
	}
	for _, p := range paths {
		if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	_ = s.run("systemctl", "daemon-reload")
	if t == USB {
		_ = s.run("udevadm", "control", "--reload")
	}
	return nil
}
