package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/macsnoeren/backup-usb/internal/app"
	"github.com/macsnoeren/backup-usb/internal/config"
	"github.com/macsnoeren/backup-usb/internal/logx"
	"github.com/macsnoeren/backup-usb/internal/msg"
)

// configDir returns the configuration directory: --config, then the
// environment override, then the platform default.
func (c *ctx) configDir() string {
	if c.opts.configDir != "" {
		return c.opts.configDir
	}
	if c.env.ConfigDir != "" {
		return c.env.ConfigDir
	}
	return config.Dir()
}

// loadConfig loads and validates the settings. Source paths are checked by
// the run itself, so a missing source gives a clear error there.
func (c *ctx) loadConfig() (*config.Config, error) {
	cfg, err := app.LoadConfig(c.configDir())
	if err != nil {
		return nil, err
	}
	if err := cfg.Validate(config.ValidateOptions{SkipPathCheck: true}); err != nil {
		return nil, app.Errorf(app.KindUsage, err, "err.config", err.Error())
	}
	c.p.Lang = msg.Lang(cfg.Language)
	return cfg, nil
}

// logger opens the log file; on failure logging is disabled with a note on
// stderr.
func (c *ctx) logger(cfg *config.Config) *logx.Logger {
	dir := c.opts.logDir
	if dir == "" {
		dir = cfg.LogDir(c.configDir())
	}
	l, err := logx.New(logx.Options{
		Dir:      dir,
		MaxBytes: int64(cfg.LogMaxSizeMB) << 20,
		Keep:     cfg.LogKeep,
		Level:    logx.ParseLevel(cfg.LogLevel),
		Now:      c.env.Now,
	})
	if err != nil {
		if !c.opts.quiet {
			fmt.Fprintln(c.env.Stderr, "usb-backup-cli: log disabled:", err)
		}
		return nil
	}
	return l
}

// deps builds the application dependencies.
func (c *ctx) deps(cfg *config.Config) *app.Deps {
	d := c.newDeps(cfg)
	c.created = append(c.created, d)
	return d
}

// unmountAll undoes the mounts made during the command.
func (c *ctx) unmountAll() {
	for _, d := range c.created {
		d.Unmount()
	}
	c.created = nil
}

func (c *ctx) newDeps(cfg *config.Config) *app.Deps {
	return &app.Deps{
		Now:       c.env.Now,
		Finder:    c.env.Finder,
		Log:       c.logger(cfg),
		ConfigDir: c.configDir(),
		Lang:      c.p.Lang,
		Sleep:     c.env.Sleep,
		LockPath:  c.env.LockPath,
		Events:    c.emitter(),
	}
}

// context returns a context that is cancelled on Ctrl+C.
func (c *ctx) context() (context.Context, context.CancelFunc) {
	var ctx context.Context
	var cancel context.CancelFunc
	if c.env.Context != nil {
		ctx, cancel = context.WithCancel(c.env.Context)
	} else {
		ctx, cancel = signal.NotifyContext(context.Background(), os.Interrupt)
	}
	c.watchCancel(cancel)
	return ctx, cancel
}
