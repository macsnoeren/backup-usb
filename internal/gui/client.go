package gui

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// Event is one JSON line from usb-backup-cli --json-stream.
type Event struct {
	Event   string          `json:"event"`
	State   string          `json:"state,omitempty"`
	Tag     string          `json:"tag,omitempty"`
	Index   int             `json:"index,omitempty"`
	Count   int             `json:"count,omitempty"`
	Pct     int             `json:"pct,omitempty"`
	Disk    string          `json:"disk,omitempty"`
	Done    uint64          `json:"done,omitempty"`
	Total   uint64          `json:"total,omitempty"`
	Code    int             `json:"code,omitempty"`
	Key     string          `json:"key,omitempty"`
	Args    []any           `json:"args,omitempty"`
	Message string          `json:"message,omitempty"`
	Report  json.RawMessage `json:"report,omitempty"`
	Output  string          `json:"output,omitempty"`
}

// CLI starts usb-backup-cli as a child process.
type CLI struct {
	// Path is the CLI executable.
	Path string
	// Prepare adjusts the command before it starts (no console window on
	// Windows).
	Prepare func(*exec.Cmd)
}

// DefaultCLI returns the CLI next to the running GUI executable.
func DefaultCLI() (CLI, error) {
	exe, err := os.Executable()
	if err != nil {
		return CLI{}, err
	}
	name := "usb-backup-cli"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return CLI{Path: filepath.Join(filepath.Dir(exe), name), Prepare: hideWindow}, nil
}

// ErrVersionMismatch means the GUI and the CLI are from different builds.
var ErrVersionMismatch = errors.New("usb-backup-cli has a different version")

// Version asks the CLI for its version.
func (c CLI) Version(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, c.Path, "version", "--json") //nolint:gosec // path of the CLI next to the GUI
	if c.Prepare != nil {
		c.Prepare(cmd)
	}
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("start %s: %w", filepath.Base(c.Path), err)
	}
	var v struct{ Version string }
	if err := json.Unmarshal(out, &v); err != nil || v.Version == "" {
		return "", fmt.Errorf("unexpected answer from %s", filepath.Base(c.Path))
	}
	return v.Version, nil
}

// CheckVersion returns an error when the CLI version differs from want.
func (c CLI) CheckVersion(ctx context.Context, want string) error {
	got, err := c.Version(ctx)
	if err != nil {
		return err
	}
	if got != want {
		return fmt.Errorf("%w: the window is %s, usb-backup-cli is %s; reinstall USB Backup", ErrVersionMismatch, want, got)
	}
	return nil
}

// Job is a running CLI command.
type Job struct {
	Events <-chan Event
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	mu     sync.Mutex
	done   chan struct{}
	result Event
}

// Start runs "usb-backup-cli <args> --json-stream". If password is not nil,
// --password-stdin is added and the password is written as the first line;
// the caller wipes it afterwards. Events arrive on Events until the result
// or error event; then the channel closes.
func (c CLI) Start(ctx context.Context, password []byte, args ...string) (*Job, error) {
	if password == nil {
		return c.StartLines(ctx, nil, args...)
	}
	return c.StartLines(ctx, [][]byte{password}, args...)
}

// StartLines is Start with several secret lines (for example the old
// password and the new one twice); each is written as one line.
func (c CLI) StartLines(ctx context.Context, lines [][]byte, args ...string) (*Job, error) {
	args = append(append([]string(nil), args...), "--json-stream")
	if lines != nil {
		args = append(args, "--password-stdin")
	}
	cmd := exec.CommandContext(ctx, c.Path, args...) //nolint:gosec // path of the CLI next to the GUI, arguments as a list
	if c.Prepare != nil {
		c.Prepare(cmd)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start %s: %w", filepath.Base(c.Path), err)
	}
	for _, password := range lines {
		line := make([]byte, len(password)+1)
		copy(line, password)
		line[len(password)] = '\n'
		_, werr := stdin.Write(line)
		clear(line)
		if werr != nil {
			_ = cmd.Process.Kill()
			return nil, fmt.Errorf("send password: %w", werr)
		}
	}
	events := make(chan Event, 64)
	j := &Job{Events: events, cmd: cmd, stdin: stdin, done: make(chan struct{})}
	go j.read(stdout, events)
	return j, nil
}

func (j *Job) read(r io.Reader, events chan<- Event) {
	defer close(j.done)
	defer close(events)
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64<<10), 64<<20)
	for sc.Scan() {
		var e Event
		if json.Unmarshal(sc.Bytes(), &e) != nil {
			continue
		}
		if e.Event == "result" || e.Event == "error" {
			j.mu.Lock()
			j.result = e
			j.mu.Unlock()
		}
		events <- e
	}
	err := j.cmd.Wait()
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.result.Event == "" {
		code := -1
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			code = ee.ExitCode()
		}
		j.result = Event{Event: "error", Code: code, Message: "usb-backup-cli stopped unexpectedly"}
		events <- j.result
	}
}

// Cancel asks the CLI to stop; it cleans up its part files itself.
func (j *Job) Cancel() {
	_, _ = io.WriteString(j.stdin, "{\"cancel\":true}\n")
}

// Wait waits for the end and returns the result or error event.
func (j *Job) Wait() Event {
	<-j.done
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.result
}

// Call runs a command and waits for its result or error event.
func (c CLI) Call(ctx context.Context, lines [][]byte, args ...string) Event {
	j, err := c.StartLines(ctx, lines, args...)
	if err != nil {
		return Event{Event: "error", Code: -1, Message: err.Error()}
	}
	for range j.Events { //nolint:revive // drain
	}
	return j.Wait()
}

// Lines splits an Output text into lines (for commands without JSON).
func Lines(s string) []string { return strings.Split(strings.TrimSpace(s), "\n") }
