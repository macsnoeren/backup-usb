package logx

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Level is a log level.
type Level int

// Levels, from most to least important.
const (
	LevelError Level = iota
	LevelWarn
	LevelInfo
	LevelDebug
)

var levelNames = []string{"ERROR", "WARN", "INFO", "DEBUG"}

// ParseLevel parses "error", "warn", "info" or "debug"; anything else is
// info.
func ParseLevel(s string) Level {
	for i, n := range levelNames {
		if strings.EqualFold(s, n) {
			return Level(i)
		}
	}
	return LevelInfo
}

func (l Level) String() string {
	if l < 0 || int(l) >= len(levelNames) {
		return "INFO"
	}
	return levelNames[l]
}

// Options configure a Logger.
type Options struct {
	// Dir is the directory of the log file.
	Dir string
	// Name is the file name, default usb-backup.log.
	Name string
	// MaxBytes rotates the file when it would grow beyond this size.
	MaxBytes int64
	// Keep is the number of old files (name.1 ... name.Keep).
	Keep int
	// Level is the most detailed level that is written.
	Level Level
	// Now returns the time stamp; default time.Now.
	Now func() time.Time
}

// Logger writes one line per event with a time stamp. Never log secrets:
// callers pass only non-sensitive values. A nil *Logger discards
// everything.
type Logger struct {
	mu   sync.Mutex
	opts Options
	path string
}

// New returns a logger. The directory is created if needed.
func New(opts Options) (*Logger, error) {
	if opts.Name == "" {
		opts.Name = "usb-backup.log"
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.MaxBytes <= 0 {
		opts.MaxBytes = 1 << 20
	}
	if err := os.MkdirAll(opts.Dir, 0o750); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}
	return &Logger{opts: opts, path: filepath.Join(opts.Dir, opts.Name)}, nil
}

// Path returns the current log file.
func (l *Logger) Path() string {
	if l == nil {
		return ""
	}
	return l.path
}

// Error logs at error level.
func (l *Logger) Error(msg string, kv ...any) { l.log(LevelError, msg, kv) }

// Warn logs at warning level.
func (l *Logger) Warn(msg string, kv ...any) { l.log(LevelWarn, msg, kv) }

// Info logs at info level.
func (l *Logger) Info(msg string, kv ...any) { l.log(LevelInfo, msg, kv) }

// Debug logs at debug level.
func (l *Logger) Debug(msg string, kv ...any) { l.log(LevelDebug, msg, kv) }

func (l *Logger) log(level Level, msg string, kv []any) {
	if l == nil || level > l.opts.Level {
		return
	}
	var b strings.Builder
	b.WriteString(l.opts.Now().Format(time.RFC3339))
	b.WriteByte(' ')
	b.WriteString(level.String())
	b.WriteByte(' ')
	b.WriteString(oneLine(msg))
	for i := 0; i+1 < len(kv); i += 2 {
		fmt.Fprintf(&b, " %v=%s", kv[i], quote(fmt.Sprint(kv[i+1])))
	}
	b.WriteByte('\n')
	line := b.String()

	l.mu.Lock()
	defer l.mu.Unlock()
	if err := l.rotateIfNeeded(int64(len(line))); err != nil {
		fmt.Fprintln(os.Stderr, "usb-backup: log rotation failed:", err)
	}
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		fmt.Fprintln(os.Stderr, "usb-backup: cannot write log:", err)
		return
	}
	if _, err := f.WriteString(line); err != nil {
		fmt.Fprintln(os.Stderr, "usb-backup: cannot write log:", err)
	}
	if err := f.Close(); err != nil {
		fmt.Fprintln(os.Stderr, "usb-backup: cannot write log:", err)
	}
}

func (l *Logger) rotateIfNeeded(add int64) error {
	st, err := os.Stat(l.path)
	if err != nil || st.Size()+add <= l.opts.MaxBytes || st.Size() == 0 {
		return nil
	}
	if l.opts.Keep <= 0 {
		return os.Remove(l.path)
	}
	oldest := fmt.Sprintf("%s.%d", l.path, l.opts.Keep)
	if err := os.Remove(oldest); err != nil && !os.IsNotExist(err) {
		return err
	}
	for i := l.opts.Keep - 1; i >= 1; i-- {
		from := fmt.Sprintf("%s.%d", l.path, i)
		if err := os.Rename(from, fmt.Sprintf("%s.%d", l.path, i+1)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return os.Rename(l.path, l.path+".1")
}

func oneLine(s string) string {
	return strings.NewReplacer("\r", `\r`, "\n", `\n`).Replace(s)
}

func quote(s string) string {
	if s == "" || strings.ContainsAny(s, " \t\r\n\"=") {
		return fmt.Sprintf("%q", s)
	}
	return s
}
