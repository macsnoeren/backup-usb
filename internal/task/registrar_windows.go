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
)

// Schtasks registers tasks with schtasks.exe.
type Schtasks struct {
	// Run runs schtasks with arguments; nil uses the real one.
	Run func(args ...string) error
}

// NewPlatformRegistrar returns the registrar for this platform.
func NewPlatformRegistrar() Registrar { return &Schtasks{} }

func schtasksPath() string {
	root := os.Getenv("SystemRoot")
	if root == "" {
		root = `C:\Windows`
	}
	return filepath.Join(root, "System32", "schtasks.exe")
}

func (s *Schtasks) run(args ...string) error {
	if s.Run != nil {
		return s.Run(args...)
	}
	cmd := exec.Command(schtasksPath(), args...) //nolint:gosec // full system path, arguments as a list
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	done := make(chan error, 1)
	out := &limitedBuffer{max: 4096}
	cmd.Stdout, cmd.Stderr = out, out
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("schtasks: %w", err)
	}
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("schtasks: %w: %s", err, out.String())
		}
		return nil
	case <-time.After(time.Minute):
		_ = cmd.Process.Kill()
		return errors.New("schtasks: timed out")
	}
}

type limitedBuffer struct {
	b   []byte
	max int
}

func (l *limitedBuffer) Write(p []byte) (int, error) {
	if room := l.max - len(l.b); room > 0 {
		l.b = append(l.b, p[:min(room, len(p))]...)
	}
	return len(p), nil
}

func (l *limitedBuffer) String() string { return string(l.b) }

// Install writes the XML as UTF-16 LE with BOM to a temporary file and
// registers it with schtasks /Create /XML /F.
func (s *Schtasks) Install(spec Spec) error {
	x, err := WindowsXML(spec)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp("", "usb-backup-task-*.xml")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(f.Name()) }()
	_, werr := f.Write(UTF16LE(x))
	cerr := f.Close()
	if err := errors.Join(werr, cerr); err != nil {
		return err
	}
	return s.run("/Create", "/TN", Name(spec.Trigger), "/XML", f.Name(), "/F")
}

// Uninstall removes a task.
func (s *Schtasks) Uninstall(t Trigger) error {
	return s.run("/Delete", "/TN", Name(t), "/F")
}
