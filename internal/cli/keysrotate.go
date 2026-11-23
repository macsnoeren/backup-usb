package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/macsnoeren/backup-usb/internal/app"
	"github.com/macsnoeren/backup-usb/internal/config"
	"github.com/macsnoeren/backup-usb/internal/secmem"
	"github.com/macsnoeren/backup-usb/internal/vault"
)

func init() {
	keysCommands["rotate"] = cmdKeysRotate
	keysCommands["rewrap"] = cmdKeysRewrap
	keysCommands["status"] = cmdKeysStatus
}

// openVaultFor asks the vault password and opens the vault; the caller
// closes the vault and wipes the password.
func (c *ctx) openVaultFor(cfg *config.Config, stdin bool, vaultPath string) (*vault.Vault, *secmem.Buffer, string, error) {
	if vaultPath == "" {
		vaultPath = cfg.Vault()
	}
	pw, err := c.readPassword(stdin)
	if err != nil {
		return nil, nil, "", err
	}
	v, err := app.OpenVault(vaultPath, pw)
	if err != nil {
		pw.Wipe()
		return nil, nil, "", err
	}
	return v, pw, vaultPath, nil
}

func cmdKeysRotate(c *ctx, args []string) error {
	fs := c.newFlags("keys rotate")
	full := fs.Bool("full", false, "also re-encrypt existing backups with a new file key (as slow as a full backup)")
	stdin := fs.Bool("password-stdin", false, "read the vault password from stdin")
	vaultPath := fs.String("vault", "", "vault `file`")
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
	deps := c.deps(cfg)
	release, err := deps.AcquireLock()
	if err != nil {
		return err
	}
	defer release()
	v, pw, vp, err := c.openVaultFor(cfg, *stdin, *vaultPath)
	if err != nil {
		return err
	}
	defer v.Close()
	defer pw.Wipe()
	res, err := app.RotateKeys(deps, cfg, v, vp, pw, *full)
	if err != nil {
		return err
	}
	if *asJSON {
		return writeJSON(c.env.Stdout, res)
	}
	fmt.Fprintln(c.env.Stdout, c.p.T("keys.rotated", res.NewKey, res.Fingerprint, res.Retired, strings.Join(res.PendingDisks, ", ")))
	return nil
}

// readYes reads one answer line from stdin.
func (c *ctx) readYes() bool {
	var b strings.Builder
	one := make([]byte, 1)
	for b.Len() < 16 {
		n, err := c.env.Stdin.Read(one)
		if n == 1 {
			if one[0] == '\n' {
				break
			}
			b.WriteByte(one[0])
		}
		if err == io.EOF || err != nil {
			break
		}
	}
	a := strings.ToLower(strings.TrimSpace(b.String()))
	return a == "y" || a == "yes" || a == "j" || a == "ja"
}

func cmdKeysRewrap(c *ctx, args []string) error {
	fs := c.newFlags("keys rewrap")
	target := fs.String("target", "", "the `disk` to update")
	full := fs.Bool("full", false, "re-encrypt with a new file key instead of only re-wrapping")
	removeOld := fs.Bool("remove-old", false, "delete retired keys no disk needs any more, without asking")
	keepOld := fs.Bool("keep-old", false, "keep retired keys, do not ask")
	stdin := fs.Bool("password-stdin", false, "read the vault password (and the answer) from stdin")
	vaultPath := fs.String("vault", "", "vault `file`")
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
	deps := c.deps(cfg)
	release, err := deps.AcquireLock()
	if err != nil {
		return err
	}
	defer release()
	v, pw, vp, err := c.openVaultFor(cfg, *stdin, *vaultPath)
	if err != nil {
		return err
	}
	defer v.Close()
	defer pw.Wipe()
	ctx, cancel := c.context()
	defer cancel()
	rep, err := app.RewrapDisk(ctx, deps, cfg, *target, v, vp, pw, *full)
	if err != nil {
		return err
	}
	if !*asJSON {
		fmt.Fprintln(c.env.Stdout, c.p.T("keys.rewrapped", rep.Disk, rep.Rewrapped, rep.Current))
	}
	for _, id := range rep.OldKeysFree {
		remove := *removeOld
		if !remove && !*keepOld {
			fmt.Fprint(c.env.Stderr, c.p.T("keys.oldFree", id)+c.p.T("keys.yesNo"))
			remove = c.readYes()
		}
		if !remove {
			if !*asJSON {
				fmt.Fprintln(c.env.Stdout, c.p.T("keys.oldKept", id))
			}
			continue
		}
		if err := v.RemoveKey(id); err != nil {
			return app.Errorf(app.KindIO, err, "err.write")
		}
		if err := v.Save(vp, pw); err != nil {
			return app.Errorf(app.KindIO, err, "err.write")
		}
		deps.Log.Info("retired key deleted", "key", id)
		if !*asJSON {
			fmt.Fprintln(c.env.Stdout, c.p.T("keys.oldRemoved", id))
		}
	}
	if *asJSON {
		return writeJSON(c.env.Stdout, rep)
	}
	return nil
}

func cmdKeysStatus(c *ctx, args []string) error {
	fs := c.newFlags("keys status")
	stdin := fs.Bool("password-stdin", false, "read the vault password from stdin")
	vaultPath := fs.String("vault", "", "vault `file`")
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
	v, pw, _, err := c.openVaultFor(cfg, *stdin, *vaultPath)
	if err != nil {
		return err
	}
	defer v.Close()
	pw.Wipe()
	if *asJSON {
		return writeJSON(c.env.Stdout, v.Meta.Keys)
	}
	for _, k := range v.Meta.Keys {
		retired, pending := "", ""
		if k.Retired != "" {
			retired = ", retired " + k.Retired
		}
		if len(k.PendingDisks) > 0 {
			pending = ", disks to update: " + strings.Join(k.PendingDisks, ", ")
		}
		fmt.Fprintln(c.env.Stdout, c.p.T("keys.statusLine", k.ID, k.Fingerprint, k.Status, k.Created, retired, pending))
	}
	fmt.Fprintln(c.env.Stdout, "break-glass", v.Meta.BreakglassFingerprint)
	return nil
}
