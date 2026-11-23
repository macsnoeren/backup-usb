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
	register(&command{name: "restore", summary: "Decrypt and unpack a backup, or single files or folders.", run: cmdRestore})
}

type multiFlag []string

func (m *multiFlag) String() string     { return fmt.Sprint(*m) }
func (m *multiFlag) Set(s string) error { *m = append(*m, s); return nil }

func cmdRestore(c *ctx, args []string) error {
	fs := c.newFlags("restore")
	var opts app.RestoreOptions
	var k keyFlags
	var files multiFlag
	fs.StringVar(&opts.Target, "target", "", "only this `disk`")
	fs.StringVar(&opts.Week, "week", "", "`period`, e.g. 2026-47 (default: newest)")
	fs.StringVar(&opts.Tag, "tag", "", "source `tag`")
	fs.Var(&files, "file", "`path` in the backup: a file or folder (repeatable); default everything")
	fs.StringVar(&opts.To, "to", "", "restore into this `folder`")
	fs.BoolVar(&opts.Original, "original", false, "restore to the original source folder (existing files are never overwritten)")
	c.addKeyFlags(fs, &k)
	asJSON := fs.Bool("json", false, "print the result as JSON")
	pos, err := c.parse(fs, args)
	if err != nil {
		return err
	}
	if err := noPositional(pos); err != nil {
		return err
	}
	if opts.Original && opts.To != "" {
		return usageErr("use either --to or --original")
	}
	opts.Files = files
	cfg, err := c.loadConfig()
	if err != nil {
		return err
	}
	ks, err := c.loadKeys(cfg, k, false)
	if err != nil {
		return err
	}
	defer ks.close()
	opts.Identities = ks.ids
	ctx, cancel := c.context()
	defer cancel()
	rep, err := app.Restore(ctx, c.deps(cfg), cfg, opts)
	if err != nil && app.KindOf(err) == app.KindUsage && len(ks.ids) == 0 && !k.passwordStdin && c.stream == nil && isNeedKey(err) {
		ks2, kerr := c.loadKeys(cfg, k, true)
		if kerr != nil {
			return kerr
		}
		defer ks2.close()
		opts.Identities = ks2.ids
		rep, err = app.Restore(ctx, c.deps(cfg), cfg, opts)
	}
	if err == nil && ks.breakglass && rep != nil && rep.Decrypted {
		_ = app.RecordBreakglassTest(c.configDir(), cfg, c.env.Now())
	}
	if *asJSON && rep != nil {
		if jerr := writeJSON(c.env.Stdout, rep); jerr != nil {
			return jerr
		}
		if err != nil {
			return quietErr{err}
		}
		return nil
	}
	if err != nil {
		return err
	}
	if !rep.FromIndex {
		fmt.Fprintln(c.env.Stderr, c.p.T("list.noIndex"))
	}
	fmt.Fprintln(c.env.Stdout, c.p.T("restore.done", rep.Files, app.HumanBytes(rep.Bytes), rep.Disk, rep.Period, rep.Tag, rep.Destination))
	for _, r := range rep.Renamed {
		fmt.Fprintln(c.env.Stdout, c.p.T("restore.renamed", r.Path, r.As))
	}
	for _, s := range rep.Skipped {
		fmt.Fprintln(c.env.Stdout, c.p.T("restore.skipped", s.Path, s.Reason))
	}
	return nil
}

func isNeedKey(err error) bool {
	var e *app.Error
	return asAppErr(err, &e) && e.Key == "err.needKey"
}
