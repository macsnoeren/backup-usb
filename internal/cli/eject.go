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
	register(&command{name: "eject", summary: "Safely remove the connected backup disk.", run: cmdEject})
}

func cmdEject(c *ctx, args []string) error {
	fs := c.newFlags("eject")
	target := fs.String("target", "", "the `disk` to remove")
	asJSON := fs.Bool("json", false, "print as JSON")
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
	ctx, cancel := c.context()
	defer cancel()
	name, err := app.Eject(ctx, c.deps(cfg), cfg, *target)
	if err != nil {
		return err
	}
	if *asJSON {
		return writeJSON(c.env.Stdout, map[string]string{"ejected": name})
	}
	fmt.Fprintln(c.env.Stdout, c.p.T("disk.ejected", name))
	return nil
}
