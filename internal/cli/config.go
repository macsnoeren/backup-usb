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
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/macsnoeren/backup-usb/internal/app"
	"github.com/macsnoeren/backup-usb/internal/config"
	"github.com/macsnoeren/backup-usb/internal/secmem"
)

func init() {
	register(&command{name: "config", summary: "Manage settings: show, set, add-source, remove-source, import, export.", run: cmdConfig})
}

var configCommands = map[string]func(c *ctx, args []string) error{
	"show": cmdConfigShow,
	"set":  cmdConfigSet,
}

func cmdConfig(c *ctx, args []string) error {
	names := make([]string, 0, len(configCommands))
	for n := range configCommands {
		names = append(names, n)
	}
	sort.Strings(names)
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		fs := c.newFlags("config")
		fs.Usage()
		fmt.Fprintln(c.helpOutput(), "\nSubcommands:", strings.Join(names, ", "))
		if len(args) > 0 && (args[0] == "-h" || args[0] == "--help") {
			return nil
		}
		return usageErr("config needs a subcommand: %s", strings.Join(names, ", "))
	}
	sub, ok := configCommands[args[0]]
	if !ok {
		return usageErr("unknown config subcommand %q", args[0])
	}
	return sub(c, args[1:])
}

// operatorKey returns the signing key from the vault (asking for the vault
// password), or nil when no keys exist yet. The vault must belong to this
// installation.
func (c *ctx) operatorKey(cfg *config.Config, stdin bool, vaultPath string) (*secmem.Buffer, error) {
	pub, err := app.SigningKey(c.configDir())
	if err != nil {
		return nil, err
	}
	if pub == nil {
		return nil, nil
	}
	if vaultPath == "" {
		vaultPath = cfg.Vault()
	}
	pw, err := c.readPassword(stdin)
	if err != nil {
		return nil, err
	}
	defer pw.Wipe()
	v, err := app.OpenVault(vaultPath, pw)
	if err != nil {
		return nil, err
	}
	defer v.Close()
	vpub, err := v.SigningPublic()
	if err != nil || !vpub.Equal(pub) {
		return nil, app.Errorf(app.KindDecrypt, errors.New("vault signing key differs from usb-backup.signing.pub"), "err.otherVault")
	}
	return v.SigningKey(), nil
}

func cmdConfigShow(c *ctx, args []string) error {
	fs := c.newFlags("config show")
	pos, err := c.parse(fs, args)
	if err != nil {
		return err
	}
	cfg, err := c.loadConfig()
	if err != nil {
		return err
	}
	if len(pos) == 1 {
		v, err := cfg.Get(pos[0])
		if err != nil {
			return usageErr("%v", err)
		}
		fmt.Fprintln(c.env.Stdout, v)
		return nil
	}
	if err := noPositional(pos); err != nil {
		return err
	}
	return writeJSON(c.env.Stdout, cfg)
}

func cmdConfigSet(c *ctx, args []string) error {
	fs := c.newFlags("config set")
	stdin := fs.Bool("password-stdin", false, "read the vault password from stdin")
	vaultPath := fs.String("vault", "", "vault `file`")
	pos, err := c.parse(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 2 {
		return usageErr("usage: config set <key> <value>; keys: %s", strings.Join(config.Keys(), ", "))
	}
	cfg, err := c.loadConfig()
	if err != nil {
		return err
	}
	oldV, newV, err := cfg.Set(pos[0], pos[1])
	if err != nil {
		return usageErr("%v", err)
	}
	if err := cfg.Validate(config.ValidateOptions{}); err != nil {
		return app.Errorf(app.KindUsage, err, "err.config", err.Error())
	}
	return c.saveConfig(cfg, *stdin, *vaultPath, func(l func(string, ...any)) {
		l("setting changed", "key", pos[0], "old", oldV, "new", newV)
	}, c.p.T("cfg.changed", pos[0], oldV, newV))
}

// saveConfig signs and saves the settings, logs and prints done.
func (c *ctx) saveConfig(cfg *config.Config, stdin bool, vaultPath string, log func(func(string, ...any)), done string) error {
	key, err := c.operatorKey(cfg, stdin, vaultPath)
	if err != nil {
		return err
	}
	defer key.Wipe()
	if err := app.SaveConfig(c.configDir(), cfg, key); err != nil {
		return err
	}
	if l := c.logger(cfg); l != nil {
		log(l.Info)
	}
	fmt.Fprintln(c.env.Stdout, done)
	if key == nil {
		fmt.Fprintln(c.env.Stderr, c.p.T("cfg.unsigned"))
	}
	return nil
}
