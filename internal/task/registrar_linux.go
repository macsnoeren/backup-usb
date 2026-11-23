package task

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/macsnoeren/backup-usb/internal/fsx"
)

// Systemd registers systemd units and udev rules.
type Systemd struct {
	// UnitDir and RulesDir default to /etc/systemd/system and
	// /etc/udev/rules.d.
	UnitDir, RulesDir string
	// Run runs a program; nil uses the real one.
	Run func(name string, args ...string) error
}

// NewPlatformRegistrar returns the registrar for this platform.
func NewPlatformRegistrar() Registrar { return &Systemd{} }

var binDirs = []string{"/usr/bin", "/bin", "/usr/sbin", "/sbin"}

func tool(name string) string {
	for _, d := range binDirs {
		p := filepath.Join(d, name)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return name
}

func (s *Systemd) dirs() (string, string) {
	u, r := s.UnitDir, s.RulesDir
	if u == "" {
		u = "/etc/systemd/system"
	}
	if r == "" {
		r = "/etc/udev/rules.d"
	}
	return u, r
}

func (s *Systemd) run(name string, args ...string) error {
	if s.Run != nil {
		return s.Run(name, args...)
	}
	cmd := exec.Command(tool(name), args...) //nolint:gosec // fixed program from system folders, arguments as a list
	cmd.Env = []string{"LC_ALL=C", "PATH=/usr/sbin:/usr/bin:/sbin:/bin"}
	done := make(chan error, 1)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		return nil
	case <-time.After(2 * time.Minute):
		_ = cmd.Process.Kill()
		return fmt.Errorf("%s: timed out", name)
	}
}

// ErrNeedRoot is returned when installing system units without root.
var ErrNeedRoot = errors.New("installing systemd units and udev rules needs root (sudo)")

// Install writes the files and enables them.
func (s *Systemd) Install(spec Spec) error {
	files, err := SystemdFiles(spec)
	if err != nil {
		return err
	}
	if s.UnitDir == "" && os.Geteuid() != 0 {
		return ErrNeedRoot
	}
	unitDir, rulesDir := s.dirs()
	for name, content := range files {
		dir := unitDir
		if name == UdevRule {
			dir = rulesDir
		}
		if err := os.MkdirAll(dir, 0o755); err != nil { //nolint:gosec // system unit folders are world-readable
			return err
		}
		if err := fsx.WriteFileAtomic(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			return err
		}
	}
	if err := s.run("systemctl", "daemon-reload"); err != nil {
		return err
	}
	if spec.Trigger == USB {
		return s.run("udevadm", "control", "--reload")
	}
	return s.run("systemctl", "enable", "--now", TimerName(spec.Trigger)+".timer")
}

// Uninstall disables and removes the files of a trigger.
func (s *Systemd) Uninstall(t Trigger) error {
	if s.UnitDir == "" && os.Geteuid() != 0 {
		return ErrNeedRoot
	}
	unitDir, rulesDir := s.dirs()
	var paths []string
	if t == USB {
		paths = []string{filepath.Join(rulesDir, UdevRule), filepath.Join(unitDir, ServiceTemplate)}
	} else {
		_ = s.run("systemctl", "disable", "--now", TimerName(t)+".timer")
		paths = []string{filepath.Join(unitDir, TimerName(t)+".timer"), filepath.Join(unitDir, TimerName(t)+".service")}
	}
	for _, p := range paths {
		if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	_ = s.run("systemctl", "daemon-reload")
	if t == USB {
		_ = s.run("udevadm", "control", "--reload")
	}
	return nil
}
