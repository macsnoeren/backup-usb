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
	"io"
	"sort"
	"strings"

	"github.com/macsnoeren/backup-usb/internal/app"
	"github.com/macsnoeren/backup-usb/internal/password"
	"github.com/macsnoeren/backup-usb/internal/secmem"
)

func init() {
	register(&command{name: "keys", summary: "Manage keys: init, passwd, rotate, rewrap, status.", run: cmdKeys})
}

var keysCommands = map[string]func(c *ctx, args []string) error{
	"init":   cmdKeysInit,
	"passwd": cmdKeysPasswd,
}

func cmdKeys(c *ctx, args []string) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		names := make([]string, 0, len(keysCommands))
		for n := range keysCommands {
			names = append(names, n)
		}
		sort.Strings(names)
		fs := c.newFlags("keys")
		fs.Usage()
		fmt.Fprintln(c.helpOutput(), "\nSubcommands:", strings.Join(names, ", "))
		if len(args) > 0 && (args[0] == "-h" || args[0] == "--help") {
			return nil
		}
		return usageErr("keys needs a subcommand: %s", strings.Join(names, ", "))
	}
	sub, ok := keysCommands[args[0]]
	if !ok {
		return usageErr("unknown keys subcommand %q", args[0])
	}
	return sub(c, args[1:])
}

func (c *ctx) helpOutput() io.Writer {
	if c.helpOut != nil {
		return c.helpOut
	}
	return c.env.Stderr
}

// passwordErr maps password input errors to a usage error.
func passwordErr(err error) error {
	return app.Errorf(app.KindUsage, err, "err.passwordInput", err.Error())
}

// readPassword asks for the existing vault password.
func (c *ctx) readPassword(stdin bool) (*secmem.Buffer, error) {
	pw, err := c.passwordSource(stdin).Password(c.p.T("keys.prompt"))
	if err != nil {
		return nil, passwordErr(err)
	}
	return pw, nil
}

// readNewPassword asks for a new vault password twice.
func (c *ctx) readNewPassword(stdin bool) (*secmem.Buffer, error) {
	pw, err := password.NewPassword(c.passwordSource(stdin), c.p.T("keys.promptNew"), c.p.T("keys.promptAgain"))
	if err != nil {
		return nil, passwordErr(err)
	}
	return pw, nil
}

func cmdKeysInit(c *ctx, args []string) error {
	fs := c.newFlags("keys init")
	vaultPath := fs.String("vault", "", "vault `file` (default: per-user recovery.vault)")
	out := fs.String("breakglass-out", "", "new `file` for the break-glass key (written once; required)")
	stdin := fs.Bool("password-stdin", false, "read the new password twice from stdin (two lines)")
	force := fs.Bool("force", false, "replace existing keys (old backups then need the old break-glass key)")
	asJSON := fs.Bool("json", false, "print the result as JSON")
	pos, err := c.parse(fs, args)
	if err != nil {
		return err
	}
	if err := noPositional(pos); err != nil {
		return err
	}
	if *out == "" {
		return usageErr("--breakglass-out is required: the break-glass key is shown only once")
	}
	cfg, err := c.loadConfig()
	if err != nil {
		return err
	}
	if *vaultPath == "" {
		*vaultPath = cfg.Vault()
	}
	pw, err := c.readNewPassword(*stdin)
	if err != nil {
		return err
	}
	defer pw.Wipe()
	f, err := app.CreateSecretFile(*out)
	if err != nil {
		return err
	}
	res, err := app.KeysInit(app.KeysInitOptions{
		ConfigDir: c.configDir(), Config: cfg, VaultPath: *vaultPath, Password: pw,
		BreakglassOut: f, Force: *force, Now: c.env.Now(),
	})
	cerr := f.Close()
	if err != nil {
		_ = removeFile(*out)
		return err
	}
	if cerr != nil {
		// The key was synced before the vault was saved; a failing close
		// after that is reported but the file stays.
		return app.Errorf(app.KindIO, cerr, "err.breakglassWrite")
	}
	if l := c.logger(cfg); l != nil {
		l.Info("keys created", "operational", res.OperationalKey, "breakglass", res.BreakglassFingerprint)
	}
	if *asJSON {
		return writeJSON(c.env.Stdout, res)
	}
	fmt.Fprintln(c.env.Stdout, c.p.T("keys.created", res.Vault, res.Recipients, res.OperationalKey, fingerprintOf(res.OperationalRecipient), res.BreakglassFingerprint, *out))
	return nil
}

// cmdKeysPasswd changes the vault password. With --password-stdin it reads
// three lines: the old password and the new one twice.
func cmdKeysPasswd(c *ctx, args []string) error {
	fs := c.newFlags("keys passwd")
	vaultPath := fs.String("vault", "", "vault `file`")
	stdin := fs.Bool("password-stdin", false, "read old password and new password twice from stdin")
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
	if *vaultPath == "" {
		*vaultPath = cfg.Vault()
	}
	release, err := c.deps(cfg).AcquireLock()
	if err != nil {
		return err
	}
	defer release()
	oldPw, err := c.readPassword(*stdin)
	if err != nil {
		return err
	}
	defer oldPw.Wipe()
	v, err := app.OpenVault(*vaultPath, oldPw)
	if err != nil {
		return err
	}
	defer v.Close()
	newPw, err := c.readNewPassword(*stdin)
	if err != nil {
		return err
	}
	defer newPw.Wipe()
	if err := v.Save(*vaultPath, newPw); err != nil {
		return app.Errorf(app.KindIO, err, "err.write")
	}
	if l := c.logger(cfg); l != nil {
		l.Info("vault password changed")
	}
	fmt.Fprintln(c.env.Stdout, c.p.T("keys.passwdDone"))
	return nil
}
