package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/macsnoeren/backup-usb/internal/app"
)

func init() {
	register(&command{name: "run", summary: "Back up all source folders to the backup disk that is due.", run: cmdRun})
}

func cmdRun(c *ctx, args []string) error {
	fs := c.newFlags("run")
	var opts app.RunOptions
	fs.StringVar(&opts.Target, "target", "", "use this `disk` (drive letter, device or mount point)")
	fs.BoolVar(&opts.AnyDisk, "any-disk", false, "accept any registered disk, ignore the odd/even rule")
	fs.StringVar(&opts.Tag, "tag", "", "back up only the source with this `tag`")
	fs.BoolVar(&opts.Force, "force", false, "ignore duplicateGuardMinutes and minIntervalHours")
	wait := fs.Int("wait", 0, "wait up to `seconds` for the disk")
	fs.BoolVar(&opts.DryRun, "dry-run", false, "show what would happen, write nothing")
	quietSkip := fs.Bool("quiet-skip", false, "no message for exit codes 2 to 4")
	asJSON := fs.Bool("json", false, "print the report as JSON")
	pos, err := c.parse(fs, args)
	if err != nil {
		return err
	}
	if err := noPositional(pos); err != nil {
		return err
	}
	if *wait < 0 || *wait > 3600 {
		return usageErr("--wait must be between 0 and 3600")
	}
	opts.Wait = time.Duration(*wait) * time.Second
	cfg, err := c.loadConfig()
	if err != nil {
		return err
	}
	ctx, cancel := c.context()
	defer cancel()
	deps := c.deps(cfg)
	rep, err := app.Run(ctx, deps, cfg, opts)
	kind := app.KindOf(err)
	skip := kind == app.KindNoDisk || kind == app.KindWrongDisk || kind == app.KindSkipped
	if *quietSkip && skip {
		return quietErr{err}
	}
	if *asJSON {
		if rep == nil {
			rep = &app.RunReport{Status: app.StatusFailed, Started: c.env.Now(), Finished: c.env.Now(), Sources: []app.SourceReport{}}
			if skip {
				rep.Status = app.StatusSkipped
			}
		}
		if err != nil && rep.Error == "" && kind != app.KindWarning {
			rep.Error = c.errText(err)
		}
		if jerr := writeJSON(c.env.Stdout, rep); jerr != nil {
			return jerr
		}
		if err != nil {
			return quietErr{err}
		}
		return nil
	}
	if rep != nil && !c.opts.quiet {
		c.printRunSummary(rep)
	}
	if kind == app.KindWarning {
		return quietErr{err}
	}
	return err
}

// quietErr carries an exit code without printing the error again.
type quietErr struct{ err error }

func (q quietErr) Error() string { return q.err.Error() }
func (q quietErr) Unwrap() error { return q.err }

func (c *ctx) printRunSummary(r *app.RunReport) {
	w := c.env.Stdout
	if r.Status == app.StatusDryRun && r.DryRun != nil {
		fmt.Fprintf(w, "%s: %s (%s)\n", c.p.T("report.disk"), r.Disk, r.Mount)
		fmt.Fprintf(w, "%s: %s\n", c.p.T("report.folder"), r.Folder)
		fmt.Fprintf(w, c.p.T("dry.size")+"\n", app.HumanBytes(r.DryRun.ExpectedBytes), app.HumanBytes(r.DryRun.RequiredBytes), app.HumanBytes(r.FreeBytes))
		if len(r.DryRun.WouldRotate) == 0 {
			fmt.Fprintln(w, c.p.T("dry.noRotate"))
		} else {
			fmt.Fprintf(w, c.p.T("dry.rotate")+"\n", strings.Join(r.DryRun.WouldRotate, ", "))
		}
		fmt.Fprintln(w, c.p.T("dry.nothingWritten"))
		return
	}
	fmt.Fprint(w, app.RenderReport(r, c.p.Lang))
}

func (c *ctx) errText(err error) string {
	var b strings.Builder
	fmt.Fprint(&b, err)
	if t := appErrText(c, err); t != "" {
		return t
	}
	return b.String()
}
