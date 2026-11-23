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

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/macsnoeren/backup-usb/internal/app"
	"github.com/macsnoeren/backup-usb/internal/task"
)

func init() {
	register(&command{name: "install", summary: "Register a scheduled task: --trigger usb (default), daily --at 02:00, weekly --day fri --at 18:00.", run: cmdInstall})
	register(&command{name: "uninstall", summary: "Remove a scheduled task: --trigger usb|daily|weekly|all.", run: cmdUninstall})
}

func (c *ctx) registrar() task.Registrar {
	if c.env.Tasks != nil {
		return c.env.Tasks
	}
	return task.NewPlatformRegistrar()
}

func (c *ctx) executable() (string, error) {
	if c.env.Executable != "" {
		return c.env.Executable, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if r, err := filepath.EvalSymlinks(exe); err == nil {
		exe = r
	}
	return exe, nil
}

func cmdInstall(c *ctx, args []string) error {
	fs := c.newFlags("install")
	trig := fs.String("trigger", "usb", "`usb`, daily or weekly")
	at := fs.String("at", "", "time of day `HH:MM` for daily and weekly")
	day := fs.String("day", "", "`weekday` for weekly: mon, tue, wed, thu, fri, sat, sun")
	pos, err := c.parse(fs, args)
	if err != nil {
		return err
	}
	if err := noPositional(pos); err != nil {
		return err
	}
	cfg, err := c.loadConfig()
	if err != nil {
		return err
	}
	exe, err := c.executable()
	if err != nil {
		return app.Errorf(app.KindIO, err, "err.write")
	}
	spec := task.Spec{Trigger: task.Trigger(*trig), At: *at, Day: *day, Command: exe, Args: []string{"run", "--quiet-skip"}}
	if spec.Trigger == task.USB {
		spec.Args = append(spec.Args, "--wait", fmt.Sprint(cfg.WaitSeconds))
		if runtime.GOOS == "windows" {
			gui := filepath.Join(filepath.Dir(exe), "usb-backup.exe")
			if st, err := os.Stat(gui); err == nil && !st.IsDir() {
				spec.Command, spec.Args = gui, []string{"--auto"}
			}
		}
		reg, err := app.LoadRegistry(c.configDir(), cfg)
		if err != nil {
			return err
		}
		for _, d := range reg.Disks {
			spec.VolumeUUIDs = append(spec.VolumeUUIDs, d.VolumeSerial)
		}
	}
	if c.opts.configDir != "" {
		spec.Args = append(spec.Args, "--config", c.opts.configDir)
	}
	if err := spec.Validate(); err != nil {
		return usageErr("%v", err)
	}
	if err := c.registrar().Install(spec); err != nil {
		return app.Errorf(app.KindIO, err, "err.install", task.Name(spec.Trigger))
	}
	if l := c.logger(cfg); l != nil {
		l.Info("task installed", "trigger", spec.Trigger, "at", spec.At, "day", spec.Day)
	}
	fmt.Fprintln(c.env.Stdout, c.p.T("task.installed", task.Name(spec.Trigger)))
	return nil
}

func cmdUninstall(c *ctx, args []string) error {
	fs := c.newFlags("uninstall")
	trig := fs.String("trigger", "usb", "`usb`, daily, weekly or all")
	pos, err := c.parse(fs, args)
	if err != nil {
		return err
	}
	if err := noPositional(pos); err != nil {
		return err
	}
	triggers := []task.Trigger{task.Trigger(*trig)}
	switch *trig {
	case "all":
		triggers = task.Triggers
	case "usb", "daily", "weekly":
	default:
		return usageErr("--trigger must be usb, daily, weekly or all")
	}
	var firstErr error
	for _, t := range triggers {
		if err := c.registrar().Uninstall(t); err != nil {
			if *trig != "all" && firstErr == nil {
				firstErr = app.Errorf(app.KindIO, err, "err.uninstall", task.Name(t))
			}
			continue
		}
		fmt.Fprintln(c.env.Stdout, c.p.T("task.removed", task.Name(t)))
	}
	return firstErr
}
