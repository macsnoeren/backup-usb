package cli

import (
	"errors"
	"fmt"

	"filippo.io/age"

	"github.com/macsnoeren/backup-usb/internal/app"
	"github.com/macsnoeren/backup-usb/internal/config"
)

func init() {
	register(&command{name: "verify", summary: "Check existing backups again; with a key, unpack them completely.", run: cmdVerify})
}

// keyFlags are --password-stdin and --identity, shared by verify, list and
// restore.
type keyFlags struct {
	passwordStdin bool
	identity      string
	vault         string
}

func (c *ctx) addKeyFlags(fs interface {
	BoolVar(*bool, string, bool, string)
	StringVar(*string, string, string, string)
}, k *keyFlags) {
	fs.BoolVar(&k.passwordStdin, "password-stdin", false, "read the vault password from stdin")
	fs.StringVar(&k.identity, "identity", "", "private key `file` (for example the break-glass key); never stored")
	fs.StringVar(&k.vault, "vault", "", "vault `file`")
}

// keys holds identities for one command; close wipes what can be wiped.
type keys struct {
	ids        []age.Identity
	breakglass bool
	close      func()
}

// loadKeys returns the identities from --identity or from the vault.
// require asks for the vault password when no --identity is given.
func (c *ctx) loadKeys(cfg *config.Config, k keyFlags, require bool) (*keys, error) {
	if k.identity != "" {
		buf, err := readSecretFile(k.identity, 64<<10)
		if err != nil {
			return nil, app.Errorf(app.KindUsage, err, "err.identity", err.Error())
		}
		defer buf.Wipe()
		ids, recipients, err := app.ParseIdentityFile(buf.Bytes())
		if err != nil {
			return nil, app.Errorf(app.KindDecrypt, err, "err.identity", err.Error())
		}
		res := &keys{ids: ids, close: func() {}}
		if rec, err := app.ReadRecipients(cfg.RecipientsPath(c.configDir())); err == nil && rec.Breakglass != "" {
			for _, r := range recipients {
				res.breakglass = res.breakglass || r == rec.Breakglass
			}
		}
		return res, nil
	}
	if !k.passwordStdin && !require {
		return &keys{close: func() {}}, nil
	}
	vaultPath := k.vault
	if vaultPath == "" {
		vaultPath = cfg.Vault()
	}
	pw, err := c.readPassword(k.passwordStdin)
	if err != nil {
		return nil, err
	}
	defer pw.Wipe()
	v, err := app.OpenVault(vaultPath, pw)
	if err != nil {
		return nil, err
	}
	ids, err := v.Identities()
	if err != nil {
		v.Close()
		return nil, app.Errorf(app.KindDecrypt, err, "err.vaultDamaged")
	}
	return &keys{ids: ids, close: v.Close}, nil
}

func cmdVerify(c *ctx, args []string) error {
	fs := c.newFlags("verify")
	var opts app.VerifyOptions
	var k keyFlags
	fs.StringVar(&opts.Target, "target", "", "check only this `disk`")
	fs.StringVar(&opts.Week, "week", "", "check this `period`, e.g. 2026-47")
	fs.StringVar(&opts.Tag, "tag", "", "check only this source `tag`")
	fs.BoolVar(&opts.All, "all", false, "check every period folder")
	c.addKeyFlags(fs, &k)
	asJSON := fs.Bool("json", false, "print the result as JSON")
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
	ks, err := c.loadKeys(cfg, k, false)
	if err != nil {
		return err
	}
	defer ks.close()
	opts.Identities = ks.ids
	ctx, cancel := c.context()
	defer cancel()
	deps := c.deps(cfg)
	rep, err := app.Verify(ctx, deps, cfg, opts)
	if err == nil && ks.breakglass && rep != nil && rep.Decrypted > 0 {
		if rerr := app.RecordBreakglassTest(c.configDir(), cfg, c.env.Now()); rerr != nil {
			deps.Log.Warn("cannot record break-glass test", "err", rerr)
		}
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
	if rep != nil && !c.opts.quiet {
		for _, a := range rep.Archives {
			var line string
			switch {
			case a.Status != app.StatusOK:
				line = c.p.T("verify.failed", a.Error)
			case a.Mode == "hash":
				line = c.p.T("verify.hash")
			default:
				line = c.p.T("verify.full", a.Files)
			}
			fmt.Fprintf(c.env.Stdout, "%s/%s: %s\n", a.Folder, a.File, line)
		}
		if err == nil && ks.breakglass && rep.Decrypted > 0 {
			fmt.Fprintln(c.env.Stdout, c.p.T("verify.breakglass"))
		}
	}
	var q quietErr
	if errors.As(err, &q) {
		return err
	}
	return err
}
