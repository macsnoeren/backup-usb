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
