package cli

import (
	"fmt"
	"strings"

	"github.com/macsnoeren/backup-usb/internal/app"
)

func init() {
	register(&command{name: "list", summary: "Browse backups: periods, sources and files (reads only the index).", run: cmdList})
}

func cmdList(c *ctx, args []string) error {
	fs := c.newFlags("list")
	var opts app.ListOptions
	var k keyFlags
	fs.StringVar(&opts.Target, "target", "", "only this `disk`")
	fs.StringVar(&opts.Week, "week", "", "`period`, e.g. 2026-47 (default: newest)")
	fs.StringVar(&opts.Tag, "tag", "", "show the files of this source `tag`")
	fs.StringVar(&opts.Path, "path", "", "show this `folder` in the backup")
	fs.StringVar(&opts.Search, "search", "", "find files whose path contains `text`")
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
	// Browsing files needs the key when the backup is encrypted.
	ks, err := c.loadKeys(cfg, k, false)
	if err != nil {
		return err
	}
	defer ks.close()
	opts.Identities = ks.ids
	ctx, cancel := c.context()
	defer cancel()
	res, err := app.List(ctx, c.deps(cfg), cfg, opts)
	if err != nil && isNeedKey(err) && len(ks.ids) == 0 && !k.passwordStdin {
		// Encrypted: ask for the vault password on the console.
		ks2, kerr := c.loadKeys(cfg, k, true)
		if kerr != nil {
			return kerr
		}
		defer ks2.close()
		opts.Identities = ks2.ids
		res, err = app.List(ctx, c.deps(cfg), cfg, opts)
	}
	if err != nil {
		return err
	}
	if *asJSON {
		return writeJSON(c.env.Stdout, res)
	}
	w := c.env.Stdout
	if opts.Tag == "" {
		fmt.Fprintln(w, c.p.T("list.periodsHead"))
		for _, p := range res.Periods {
			lock := ""
			if p.Encrypted {
				lock = " (encrypted)"
			}
			fmt.Fprintf(w, "%-10s %-9s %-10s %s%s\n", p.Period, p.Disk, p.Folder, strings.Join(p.Tags, ", "), lock)
		}
		return nil
	}
	if !res.FromIndex {
		fmt.Fprintln(c.env.Stderr, c.p.T("list.noIndex"))
	}
	for _, e := range res.Entries {
		if e.Dir {
			fmt.Fprintf(w, "%-12s %-20s %s/\n", "-", "", e.Path)
			continue
		}
		fmt.Fprintf(w, "%-12s %-20s %s\n", app.HumanBytes(e.Size), e.Modified.Local().Format("2006-01-02 15:04"), e.Path)
	}
	return nil
}
