package cli

import (
	"fmt"
	"strings"

	"github.com/macsnoeren/backup-usb/internal/app"
)

func init() {
	register(&command{name: "status", summary: "Show the connected backup disks: health, free space, folders, last backup.", run: cmdStatus})
}

func cmdStatus(c *ctx, args []string) error {
	fs := c.newFlags("status")
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
	rep, err := app.Status(ctx, c.deps(cfg), cfg)
	if err != nil {
		return err
	}
	if *asJSON {
		return writeJSON(c.env.Stdout, rep)
	}
	w := c.env.Stdout
	fmt.Fprintln(w, c.p.T("status.period", rep.Period, strings.Join(rep.Due, " / ")))
	if len(rep.Disks) == 0 {
		fmt.Fprintln(w, c.p.T("status.noDisks"))
	}
	for _, d := range rep.Disks {
		last := c.p.T("status.never")
		if d.LastBackup != nil {
			last = d.LastBackup.Local().Format("2006-01-02 15:04")
		}
		mark := ""
		if d.Due {
			mark = c.p.T("status.dueMark")
		}
		fmt.Fprintln(w, c.p.T("status.disk", d.Name, d.Mount, d.Health, app.HumanBytes(d.FreeBytes), app.HumanBytes(d.TotalBytes), d.Folders, last, mark))
		if d.NeedsUpdate {
			fmt.Fprintln(w, c.p.T("status.needsUpdate"))
		}
	}
	switch {
	case !rep.Encrypted:
		fmt.Fprintln(w, c.p.T("status.notEncrypted"))
	case rep.BreakglassTested == nil:
		fmt.Fprintln(w, c.p.T("status.bgNever"))
	default:
		fmt.Fprintln(w, c.p.T("status.bgTested", rep.BreakglassTested.Local().Format("2006-01-02")))
	}
	return nil
}
