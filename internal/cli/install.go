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
	register(&command{name: "install", summary: "Start a backup when a registered disk is plugged in, or (--trigger login, Windows) the window when a user logs in.", run: cmdInstall})
	register(&command{name: "uninstall", summary: "Stop starting a backup when a disk is plugged in; --trigger all also removes tasks of older versions.", run: cmdUninstall})
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
	trig := fs.String("trigger", "usb", "`usb` or login")
	check := fs.Bool("check", false, "only show whether it is switched on")
	asJSON := fs.Bool("json", false, "with --check: print as JSON")
	pos, err := c.parse(fs, args)
	if err != nil {
		return err
	}
	if err := noPositional(pos); err != nil {
		return err
	}
	if *check {
		usb, login := c.registrar().Installed(task.USB), c.registrar().Installed(task.Login)
		if *asJSON {
			return writeJSON(c.env.Stdout, map[string]bool{"usb": usb, "login": login})
		}
		key := "task.off"
		if usb {
			key = "task.on"
		}
		fmt.Fprintln(c.env.Stdout, c.p.T(key))
		if login {
			fmt.Fprintln(c.env.Stdout, c.p.T("task.loginOn"))
		}
		return nil
	}
	cfg, err := c.loadConfig()
	if err != nil {
		return err
	}
	exe, err := c.executable()
	if err != nil {
		return app.Errorf(app.KindIO, err, "err.write")
	}
	spec := task.Spec{Trigger: task.Trigger(*trig), Command: exe, Args: []string{"run", "--quiet-skip"}}
	spec.Args = append(spec.Args, "--wait", fmt.Sprint(cfg.WaitSeconds))
	if runtime.GOOS == "windows" {
		gui := filepath.Join(filepath.Dir(exe), "usb-backup.exe")
		if st, err := os.Stat(gui); err == nil && !st.IsDir() {
			spec.Command, spec.Args, spec.Stays = gui, []string{"--auto"}, cfg.TrayIcon
		}
	}
	if spec.Trigger == task.Login {
		if spec, err = loginSpec(exe, cfg.TrayIcon); err != nil {
			return err
		}
	} else {
		reg, err := app.LoadRegistry(c.configDir(), cfg)
		if err != nil {
			return err
		}
		for _, d := range reg.Disks {
			spec.VolumeUUIDs = append(spec.VolumeUUIDs, d.VolumeSerial)
		}
		if c.opts.configDir != "" {
			spec.Args = append(spec.Args, "--config", c.opts.configDir)
		}
	}
	if err := spec.Validate(); err != nil {
		return usageErr("%v", err)
	}
	if err := c.registrar().Install(spec); err != nil {
		return app.Errorf(app.KindIO, err, "err.install", task.Name(spec.Trigger))
	}
	if l := c.logger(cfg); l != nil {
		l.Info("task installed", "trigger", spec.Trigger)
	}
	fmt.Fprintln(c.env.Stdout, c.p.T("task.installed", task.Name(spec.Trigger)))
	return nil
}

// loginSpec starts the window next to the CLI, hidden. Without trayIcon a
// hidden window would end at once.
func loginSpec(exe string, trayIcon bool) (task.Spec, error) {
	if !trayIcon {
		return task.Spec{}, app.Errorf(app.KindUsage, nil, "err.trayOff")
	}
	gui := filepath.Join(filepath.Dir(exe), "usb-backup.exe")
	if st, err := os.Stat(gui); err != nil || !st.Mode().IsRegular() {
		return task.Spec{}, app.Errorf(app.KindUsage, err, "err.noWindow")
	}
	return task.Spec{Trigger: task.Login, Command: gui, Args: []string{"--tray"}}, nil
}

func cmdUninstall(c *ctx, args []string) error {
	fs := c.newFlags("uninstall")
	trig := fs.String("trigger", "usb", "`usb`, login, or all (also tasks of older versions)")
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
	case "usb", "login", "daily", "weekly":
	default:
		return usageErr("--trigger must be usb, login, daily, weekly or all")
	}
	var firstErr error
	for _, t := range triggers {
		if *trig == "all" && t == task.Login && !c.registrar().Installed(t) {
			continue // most computers never had it
		}
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
