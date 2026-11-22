package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Dir returns the platform configuration directory: C:\ProgramData\USB
// Backup on Windows, /etc/usb-backup for root on Linux, otherwise
// ~/.config/usb-backup.
func Dir() string {
	if runtime.GOOS == "windows" {
		base := os.Getenv("ProgramData")
		if base == "" {
			base = `C:\ProgramData`
		}
		return filepath.Join(base, "USB Backup")
	}
	if os.Geteuid() == 0 {
		return "/etc/usb-backup"
	}
	return filepath.Join(userConfigBase(), "usb-backup")
}

// DefaultVaultPath is %APPDATA%\USB Backup\recovery.vault on Windows and
// ~/.config/usb-backup/recovery.vault elsewhere.
func DefaultVaultPath() string {
	if runtime.GOOS == "windows" {
		base := os.Getenv("APPDATA")
		if base == "" {
			base = userConfigBase()
		}
		return filepath.Join(base, "USB Backup", VaultFileName)
	}
	return filepath.Join(userConfigBase(), "usb-backup", VaultFileName)
}

func userConfigBase() string {
	if d, err := os.UserConfigDir(); err == nil {
		return d
	}
	if h, err := os.UserHomeDir(); err == nil {
		return filepath.Join(h, ".config")
	}
	return "."
}

func defaultSources() []Source {
	if runtime.GOOS == "windows" {
		return []Source{{Tag: "Data", Path: `C:\Data`}}
	}
	return []Source{}
}

// expandEnv expands %VAR% (Windows style) and $VAR in p.
func expandEnv(p string) string {
	for {
		i := strings.IndexByte(p, '%')
		if i < 0 {
			break
		}
		j := strings.IndexByte(p[i+1:], '%')
		if j < 0 {
			break
		}
		name := p[i+1 : i+1+j]
		p = p[:i] + os.Getenv(name) + p[i+2+j:]
	}
	return os.ExpandEnv(p)
}
