// Package cli implements the usb-backup-cli commands, flags and exit codes.
package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/macsnoeren/backup-usb/internal/app"
	"github.com/macsnoeren/backup-usb/internal/msg"
	"github.com/macsnoeren/backup-usb/internal/password"
	"github.com/macsnoeren/backup-usb/internal/secmem"
	"github.com/macsnoeren/backup-usb/internal/task"
	"github.com/macsnoeren/backup-usb/internal/volume"
)

// Version is the program version, set by main.
var Version = "dev"

// Env holds everything a command touches outside the process. Run builds the
// production Env; tests build their own.
type Env struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer

	// Now returns the current time.
	Now func() time.Time
	// Finder finds volumes.
	Finder volume.Finder
	// ConfigDir overrides the platform configuration directory.
	ConfigDir string
	// Getenv reads environment variables (never used for secrets).
	Getenv func(string) string
	// Sleep replaces waiting between disk searches (tests).
	Sleep func(ctx context.Context, d time.Duration) error
	// Context, if set, is the parent context of every command (tests).
	Context context.Context
	// Console replaces the console password prompt (tests).
	Console password.Source
	// LockPath overrides the single-instance lock (tests).
	LockPath string
	// Tasks replaces the task registrar (tests).
	Tasks task.Registrar
	// Executable replaces the path of this program (tests).
	Executable string
}

// Run executes the command line in args (args[0] is the program name) and
// returns the process exit code.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	// Keep secrets out of core dumps and crash reports. Failure is not
	// fatal: the process is short-lived either way.
	_ = secmem.ProtectProcess()
	env := &Env{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
		Now:    time.Now,
		Finder: platformFinder(),
		Getenv: os.Getenv,
	}
	if err := applyFakes(env); err != nil {
		fmt.Fprintln(stderr, "usb-backup-cli:", err)
		return ExitUsage
	}
	return env.Run(args)
}

// command is one subcommand.
type command struct {
	name    string
	summary string // English, for "help <command>"
	run     func(c *ctx, args []string) error
}

var commands = map[string]*command{}

func register(c *command) { commands[c.name] = c }

func init() {
	register(&command{name: "version", summary: "Show the version.", run: cmdVersion})
	register(&command{name: "help", summary: "Show help.", run: cmdHelp})
}

// ctx is the state of one invocation.
type ctx struct {
	env  *Env
	p    msg.Printer
	opts globalOptions
	// helpOut receives flag help; stderr unless "help <command>" was used.
	helpOut io.Writer
	// stream is set with --json-stream.
	stream *streamer
}

type globalOptions struct {
	configDir string
	logDir    string
	ui        bool
	noUI      bool
	quiet     bool
}

// Run executes args with this environment and returns the exit code.
func (e *Env) Run(args []string) int {
	c := &ctx{env: e, p: msg.Printer{Lang: msg.Default}}
	if len(args) < 2 {
		fmt.Fprint(e.Stdout, c.p.T("help.usage"))
		return ExitOK
	}
	name := args[1]
	switch name {
	case "-h", "--help", "-help":
		name = "help"
	case "--version", "-version":
		name = "version"
	}
	cmd, ok := commands[name]
	if !ok {
		fmt.Fprintln(e.Stderr, "usb-backup-cli:", c.p.T("err.unknownCommand", name))
		return ExitUsage
	}
	rest := args[2:]
	stream := false
	for i, a := range rest {
		if a == "--json-stream" || a == "-json-stream" {
			stream = true
			rest = append(append([]string(nil), rest[:i]...), rest[i+1:]...)
			break
		}
	}
	if stream {
		return e.runStream(c, cmd, rest)
	}
	err := cmd.run(c, rest)
	if errors.Is(err, flag.ErrHelp) {
		return ExitOK
	}
	var q quietErr
	if err != nil && !errors.As(err, &q) {
		c.printError(err)
	}
	return ExitCode(err)
}

func (c *ctx) printError(err error) {
	if t := appErrText(c, err); t != "" {
		fmt.Fprintln(c.env.Stderr, "usb-backup-cli:", t)
		return
	}
	fmt.Fprintln(c.env.Stderr, "usb-backup-cli:", err)
}

// appErrText translates an *app.Error, with its technical cause.
func appErrText(c *ctx, err error) string {
	var ae *app.Error
	if !errors.As(err, &ae) || !msg.Has(ae.Key) {
		return ""
	}
	text := c.p.T(ae.Key, ae.Args...)
	if ae.Err != nil && ae.Kind != app.KindUsage && ae.Kind != app.KindWarning {
		text += " (" + ae.Err.Error() + ")"
	}
	return text
}

// newFlags returns a flag set with the global flags registered.
func (c *ctx) newFlags(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	out := c.helpOut
	if out == nil {
		out = c.env.Stderr
	}
	fs.SetOutput(out)
	fs.StringVar(&c.opts.configDir, "config", "", "configuration `directory`")
	fs.StringVar(&c.opts.logDir, "log", "", "log `directory`")
	fs.BoolVar(&c.opts.ui, "ui", false, "show message boxes (Windows)")
	fs.BoolVar(&c.opts.noUI, "no-ui", false, "never show message boxes")
	fs.BoolVar(&c.opts.quiet, "quiet", false, "print less")
	fs.Usage = func() {
		summary := ""
		if cmd, ok := commands[name]; ok {
			summary = cmd.summary
		}
		fmt.Fprintf(out, c.p.T("help.command"), name, summary)
		fs.PrintDefaults()
	}
	return fs
}

// passwordFlags are refused everywhere: secrets never come from arguments.
var passwordFlags = []string{"password", "passphrase", "pass", "pw", "wachtwoord", "identity-key", "secret"}

// parse parses flags and positional arguments in any order and returns the
// positional arguments.
func (c *ctx) parse(fs *flag.FlagSet, args []string) ([]string, error) {
	for _, a := range args {
		if a == "--" {
			break
		}
		name := strings.TrimLeft(a, "-")
		if name == a {
			continue
		}
		name, _, _ = strings.Cut(name, "=")
		for _, p := range passwordFlags {
			if strings.EqualFold(name, p) {
				return nil, app.Errorf(app.KindUsage, nil, "err.passwordArgument")
			}
		}
	}
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return nil, err
			}
			return nil, app.Errorf(app.KindUsage, err, "err.usage", err.Error())
		}
		args = fs.Args()
		if len(args) == 0 {
			return pos, nil
		}
		if args[0] == "--" {
			return append(pos, args[1:]...), nil
		}
		pos = append(pos, args[0])
		args = args[1:]
	}
}

func usageErr(format string, args ...any) error {
	m := fmt.Sprintf(format, args...)
	return app.Errorf(app.KindUsage, errors.New(m), "err.usage", m)
}

func noPositional(pos []string) error {
	if len(pos) > 0 {
		return usageErr("unexpected argument %q", pos[0])
	}
	return nil
}

func cmdVersion(c *ctx, args []string) error {
	fs := c.newFlags("version")
	asJSON := fs.Bool("json", false, "output as JSON")
	pos, err := c.parse(fs, args)
	if err != nil {
		return err
	}
	if err := noPositional(pos); err != nil {
		return err
	}
	if *asJSON {
		return writeJSON(c.env.Stdout, map[string]string{
			"version": Version, "go": runtime.Version(), "os": runtime.GOOS, "arch": runtime.GOARCH,
		})
	}
	fmt.Fprintln(c.env.Stdout, c.p.T("version", Version, runtime.Version(), runtime.GOOS, runtime.GOARCH))
	return nil
}

func cmdHelp(c *ctx, args []string) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") || args[0] == "help" {
		fmt.Fprint(c.env.Stdout, c.p.T("help.usage"))
		return nil
	}
	cmd, ok := commands[args[0]]
	if !ok {
		return app.Errorf(app.KindUsage, nil, "err.unknownCommand", args[0])
	}
	// Let the command register its flags and print them.
	c.helpOut = c.env.Stdout
	_ = cmd.run(c, []string{"-h"})
	return nil
}

// commandNames returns the registered command names, sorted.
func commandNames() []string {
	out := make([]string, 0, len(commands))
	for n := range commands {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}
