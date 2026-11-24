// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Maurice Snoeren
//
// This file is part of USB Backup.
//
// USB Backup is free software: you can redistribute it and/or modify it under
// the terms of the GNU General Public License as published by the Free Software
// Foundation, either version 3 of the License, or (at your option) any later
// version.
//
// USB Backup is distributed in the hope that it will be useful, but WITHOUT ANY
// WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A
// PARTICULAR PURPOSE. See the GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License along with
// USB Backup. If not, see <https://www.gnu.org/licenses/>.

package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/macsnoeren/backup-usb/internal/week"
)

// File names in the configuration directory.
const (
	FileName          = "usb-backup.json"
	SignatureSuffix   = ".sig"
	DefaultDisksFile  = "usb-backup.disks.json"
	DefaultRecipients = "usb-backup.recipients"
	SigningPubFile    = "usb-backup.signing.pub"
	LogFileName       = "usb-backup.log"
	VaultFileName     = "recovery.vault"
	// MaxFileSize bounds every JSON file read from the configuration
	// directory.
	MaxFileSize = 1 << 20
)

// Source is one folder to back up.
type Source struct {
	Tag     string   `json:"tag"`
	Path    string   `json:"path"`
	Exclude []string `json:"exclude,omitempty"`
}

// Config holds every setting from the settings table in the design. The
// JSON names are the setting keys.
type Config struct {
	// Sources
	Sources        []Source `json:"sources"`
	Exclude        []string `json:"exclude"`
	FollowSymlinks bool     `json:"followSymlinks"`
	// Target
	TargetFolder  string `json:"targetFolder"`
	FolderPattern string `json:"folderPattern"`
	ArchiveName   string `json:"archiveName"`
	// Schedule
	Schedule              week.Schedule `json:"schedule"`
	Rotation              week.Rotation `json:"rotation"`
	MinIntervalHours      int           `json:"minIntervalHours"`
	DuplicateGuardMinutes int           `json:"duplicateGuardMinutes"`
	// Retention
	KeepPerDisk     int    `json:"keepPerDisk"`
	KeepMinDays     int    `json:"keepMinDays"`
	FreeSpaceAction string `json:"freeSpaceAction"`
	// Zip
	CompressionLevel int      `json:"compressionLevel"`
	StoreExtensions  []string `json:"storeExtensions"`
	LockedFiles      string   `json:"lockedFiles"`
	// Verification
	VerifyAfterBackup      bool `json:"verifyAfterBackup"`
	VerifyPreviousOnInsert bool `json:"verifyPreviousOnInsert"`
	MinFreePercent         int  `json:"minFreePercent"`
	// Disks
	DisksFile             string `json:"disksFile"`
	RequireHardwareSerial bool   `json:"requireHardwareSerial"`
	WaitSeconds           int    `json:"waitSeconds"`
	EjectAfter            bool   `json:"ejectAfter"`
	// Encryption
	Encryption             string `json:"encryption"`
	RecipientsFile         string `json:"recipientsFile"`
	VaultPath              string `json:"vaultPath"`
	OperatorTimeoutMinutes int    `json:"operatorTimeoutMinutes"`
	// Notifications
	Notify     string `json:"notify"`
	Language   string `json:"language"`
	WebhookURL string `json:"webhookUrl"`
	// Log
	LogPath      string `json:"logPath"`
	LogMaxSizeMB int    `json:"logMaxSizeMB"`
	LogKeep      int    `json:"logKeep"`
	LogLevel     string `json:"logLevel"`
}

// Enumerated values.
const (
	FreeSpaceStop         = "stop"
	FreeSpaceDeleteOldest = "deleteOldest"
	LockedSkip            = "skip"
	LockedFail            = "fail"
	EncryptionAuto        = "auto" // age if the recipients file exists
	EncryptionAge         = "age"
	EncryptionNone        = "none"
	NotifyAlways          = "always"
	NotifyProblems        = "problemsOnly"
	NotifyNever           = "never"
)

// Defaults returns the default settings.
func Defaults() *Config {
	return &Config{
		Sources:                defaultSources(),
		Exclude:                []string{"*.tmp", "~$*", "Thumbs.db", "desktop.ini"},
		FollowSymlinks:         false,
		TargetFolder:           "Backup",
		FolderPattern:          `{year}\{week}`,
		ArchiveName:            "{tag}.zip",
		Schedule:               week.ISOWeek,
		Rotation:               week.OddEven,
		MinIntervalHours:       20,
		DuplicateGuardMinutes:  10,
		KeepPerDisk:            25,
		KeepMinDays:            0,
		FreeSpaceAction:        FreeSpaceStop,
		CompressionLevel:       6,
		StoreExtensions:        []string{"jpg", "png", "mp4", "mov", "zip", "7z", "rar", "docx", "xlsx", "pptx"},
		LockedFiles:            LockedSkip,
		VerifyAfterBackup:      true,
		VerifyPreviousOnInsert: false,
		MinFreePercent:         5,
		DisksFile:              DefaultDisksFile,
		RequireHardwareSerial:  false,
		WaitSeconds:            30,
		EjectAfter:             true,
		Encryption:             EncryptionAuto,
		RecipientsFile:         DefaultRecipients,
		VaultPath:              "",
		OperatorTimeoutMinutes: 5,
		Notify:                 NotifyAlways,
		Language:               "en",
		WebhookURL:             "",
		LogPath:                "",
		LogMaxSizeMB:           1,
		LogKeep:                5,
		LogLevel:               "info",
	}
}

// Path returns the settings file in dir.
func Path(dir string) string { return filepath.Join(dir, FileName) }

// Load reads the settings from dir. A missing file gives the defaults; an
// unknown key gives an error naming the key.
func Load(dir string) (*Config, error) {
	data, err := ReadLimited(Path(dir), MaxFileSize)
	if errors.Is(err, os.ErrNotExist) {
		return Defaults(), nil
	}
	if err != nil {
		return nil, err
	}
	return Parse(data)
}

// Parse decodes settings over the defaults.
func Parse(data []byte) (*Config, error) {
	c := Defaults()
	if len(bytes.TrimSpace(data)) == 0 {
		return c, nil
	}
	// Sources replace the default list instead of being merged into it.
	c.Sources = nil
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(c); err != nil {
		if key, ok := strings.CutPrefix(err.Error(), "json: unknown field "); ok {
			return nil, fmt.Errorf("unknown setting %s", key)
		}
		return nil, fmt.Errorf("settings file: %w", err)
	}
	if dec.More() {
		return nil, errors.New("settings file: unexpected data after the settings object")
	}
	if !bytes.Contains(data, []byte(`"sources"`)) {
		c.Sources = defaultSources()
	}
	return c, nil
}

// Marshal encodes the settings as indented JSON with a trailing newline.
func (c *Config) Marshal() ([]byte, error) {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode settings: %w", err)
	}
	return append(data, '\n'), nil
}

// Clone returns a deep copy.
func (c *Config) Clone() *Config {
	data, err := json.Marshal(c)
	if err != nil {
		panic(err)
	}
	out := &Config{}
	if err := json.Unmarshal(data, out); err != nil {
		panic(err)
	}
	return out
}

// ReadLimited reads a file of at most max bytes.
func ReadLimited(path string, maxSize int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }() // read-only
	data, err := io.ReadAll(io.LimitReader(f, maxSize+1))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", filepath.Base(path), err)
	}
	if int64(len(data)) > maxSize {
		return nil, fmt.Errorf("%s is larger than %d bytes", filepath.Base(path), maxSize)
	}
	return data, nil
}

// Resolve returns p relative to the configuration directory if it is not
// absolute.
func Resolve(dir, p string) string {
	if p == "" || filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(dir, p)
}

// DisksPath returns the registration list file.
func (c *Config) DisksPath(dir string) string { return Resolve(dir, c.DisksFile) }

// RecipientsPath returns the recipients file.
func (c *Config) RecipientsPath(dir string) string { return Resolve(dir, c.RecipientsFile) }

// LogDir returns the directory of the log file. On Windows the default is
// the "logs" folder in the configuration folder: the configuration folder
// itself is writable for administrators only, the logs folder also for the
// users who run backups.
func (c *Config) LogDir(dir string) string {
	if c.LogPath == "" {
		if runtime.GOOS == "windows" {
			return filepath.Join(dir, "logs")
		}
		return dir
	}
	return Resolve(dir, c.LogPath)
}

// Vault returns the vault path, the per-user default when empty.
func (c *Config) Vault() string {
	if c.VaultPath != "" {
		return expandEnv(c.VaultPath)
	}
	return DefaultVaultPath()
}
