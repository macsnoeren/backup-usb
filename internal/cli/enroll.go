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

	"github.com/macsnoeren/backup-usb/internal/app"
)

func init() {
	register(&command{name: "enroll", summary: "Register a disk: write its signed ID file and record its serial numbers.", run: cmdEnroll})
}

func cmdEnroll(c *ctx, args []string) error {
	fs := c.newFlags("enroll")
	var o app.EnrollOptions
	fs.StringVar(&o.Name, "name", "", "disk `name`, e.g. BACKUP1")
	fs.StringVar(&o.Weeks, "weeks", "", "`odd` or `even` (for oddEven rotation)")
	fs.BoolVar(&o.Force, "force", false, "replace an existing registration")
	stdin := fs.Bool("password-stdin", false, "read the vault password from stdin")
	vaultPath := fs.String("vault", "", "vault `file`")
	asJSON := fs.Bool("json", false, "print the registration as JSON")
	pos, err := c.parse(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 1 || o.Name == "" {
		return usageErr("usage: enroll <disk> --name BACKUP1 --weeks odd|even")
	}
	o.Target = pos[0]
	cfg, err := c.loadConfig()
	if err != nil {
		return err
	}
	key, err := c.operatorKey(cfg, *stdin, *vaultPath)
	if err != nil {
		return err
	}
	defer key.Wipe()
	o.Key = key
	ctx, cancel := c.context()
	defer cancel()
	r, err := app.Enroll(ctx, c.deps(cfg), cfg, o)
	if err != nil {
		return err
	}
	if *asJSON {
		return writeJSON(c.env.Stdout, r)
	}
	weeks := r.Weeks
	if weeks == "" {
		weeks = string(cfg.Rotation)
	}
	fmt.Fprintln(c.env.Stdout, c.p.T("disk.enrolled", r.Name, weeks, r.ID))
	return nil
}
