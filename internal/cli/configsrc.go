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
	"strings"

	"github.com/macsnoeren/backup-usb/internal/app"
	"github.com/macsnoeren/backup-usb/internal/config"
)

func init() {
	configCommands["add-source"] = cmdConfigAddSource
	configCommands["remove-source"] = cmdConfigRemoveSource
	configCommands["export"] = cmdConfigExport
	configCommands["import"] = cmdConfigImport
}

func cmdConfigAddSource(c *ctx, args []string) error {
	fs := c.newFlags("config add-source")
	tag := fs.String("tag", "", "short `name` of the source; becomes the archive name")
	path := fs.String("path", "", "the `folder` to back up")
	var excludes multiFlag
	fs.Var(&excludes, "exclude", "exclude `pattern` for this source (repeatable)")
	stdin := fs.Bool("password-stdin", false, "read the vault password from stdin")
	vaultPath := fs.String("vault", "", "vault `file`")
	pos, err := c.parse(fs, args)
	if err != nil {
		return err
	}
	if err := noPositional(pos); err != nil {
		return err
	}
	if *tag == "" || *path == "" {
		return usageErr("usage: config add-source --tag <tag> --path <folder>")
	}
	cfg, err := c.loadConfig()
	if err != nil {
		return err
	}
	abs, err := filepath.Abs(*path)
	if err != nil {
		return usageErr("%v", err)
	}
	for _, s := range cfg.Sources {
		if strings.EqualFold(s.Tag, *tag) {
			return usageErr("tag %q is already used", *tag)
		}
	}
	if st, err := os.Stat(abs); err != nil || !st.IsDir() {
		return app.Errorf(app.KindUsage, err, "err.config", fmt.Sprintf("%q does not exist or is not a folder", abs))
	}
	cfg.Sources = append(cfg.Sources, config.Source{Tag: *tag, Path: abs, Exclude: excludes})
	if err := cfg.Validate(config.ValidateOptions{SkipPathCheck: true}); err != nil {
		return app.Errorf(app.KindUsage, err, "err.config", err.Error())
	}
	return c.saveConfig(cfg, *stdin, *vaultPath, func(l func(string, ...any)) {
		l("source added", "tag", *tag, "path", abs)
	}, c.p.T("cfg.sourceAdded", *tag, abs))
}

func cmdConfigRemoveSource(c *ctx, args []string) error {
	fs := c.newFlags("config remove-source")
	tag := fs.String("tag", "", "`tag` of the source to remove")
	stdin := fs.Bool("password-stdin", false, "read the vault password from stdin")
	vaultPath := fs.String("vault", "", "vault `file`")
	pos, err := c.parse(fs, args)
	if err != nil {
		return err
	}
	if *tag == "" && len(pos) == 1 {
		*tag = pos[0]
	} else if err := noPositional(pos); err != nil {
		return err
	}
	cfg, err := c.loadConfig()
	if err != nil {
		return err
	}
	kept := cfg.Sources[:0:0]
	for _, s := range cfg.Sources {
		if !strings.EqualFold(s.Tag, *tag) {
			kept = append(kept, s)
		}
	}
	if len(kept) == len(cfg.Sources) {
		return app.Errorf(app.KindUsage, nil, "err.unknownTag", *tag)
	}
	cfg.Sources = kept
	return c.saveConfig(cfg, *stdin, *vaultPath, func(l func(string, ...any)) {
		l("source removed", "tag", *tag)
	}, c.p.T("cfg.sourceRemoved", *tag))
}

// cmdConfigExport writes the settings (never keys or the vault) to a file
// or stdout.
func cmdConfigExport(c *ctx, args []string) error {
	fs := c.newFlags("config export")
	pos, err := c.parse(fs, args)
	if err != nil {
		return err
	}
	cfg, err := c.loadConfig()
	if err != nil {
		return err
	}
	data, err := cfg.Marshal()
	if err != nil {
		return app.Errorf(app.KindIO, err, "err.write")
	}
	if len(pos) == 0 || pos[0] == "-" {
		_, err := c.env.Stdout.Write(data)
		return err
	}
	if len(pos) > 1 {
		return usageErr("usage: config export [file]")
	}
	f, err := os.OpenFile(pos[0], os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644) //nolint:gosec // settings contain no secrets
	if err != nil {
		return app.Errorf(app.KindIO, err, "err.restoreWrite", pos[0])
	}
	_, werr := f.Write(data)
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		return app.Errorf(app.KindIO, werr, "err.write")
	}
	fmt.Fprintln(c.env.Stdout, c.p.T("cfg.exported", pos[0]))
	return nil
}

// cmdConfigImport replaces the settings with a file made by export. The
// file is checked completely, then signed.
func cmdConfigImport(c *ctx, args []string) error {
	fs := c.newFlags("config import")
	stdin := fs.Bool("password-stdin", false, "read the vault password from stdin")
	vaultPath := fs.String("vault", "", "vault `file`")
	pos, err := c.parse(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 1 {
		return usageErr("usage: config import <file>")
	}
	data, err := config.ReadLimited(pos[0], config.MaxFileSize)
	if err != nil {
		return app.Errorf(app.KindUsage, err, "err.config", err.Error())
	}
	cfg, err := config.Parse(data)
	if err != nil {
		return app.Errorf(app.KindUsage, err, "err.config", err.Error())
	}
	if err := cfg.Validate(config.ValidateOptions{}); err != nil {
		return app.Errorf(app.KindUsage, err, "err.config", err.Error())
	}
	// The current settings must be valid too (signature), so an import
	// cannot be used to hide a tampered state.
	if _, err := c.loadConfig(); err != nil && app.KindOf(err) == app.KindTampered {
		return err
	}
	c.p.Lang = msgLang(cfg.Language)
	return c.saveConfig(cfg, *stdin, *vaultPath, func(l func(string, ...any)) {
		l("settings imported", "file", pos[0])
	}, c.p.T("cfg.imported", pos[0]))
}
