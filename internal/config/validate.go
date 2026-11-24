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
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/macsnoeren/backup-usb/internal/week"
)

// ValidateOptions tune Validate.
type ValidateOptions struct {
	// SkipPathCheck does not check that source paths exist.
	SkipPathCheck bool
}

var (
	tagRe       = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	extRe       = regexp.MustCompile(`^[A-Za-z0-9]{1,16}$`)
	safeCompRe  = regexp.MustCompile(`^[A-Za-z0-9 _.-]{1,64}$`)
	archiveRe   = regexp.MustCompile(`^[A-Za-z0-9_.-]*\{tag\}[A-Za-z0-9_.-]*\.zip$`)
	logLevels   = []string{"error", "warn", "info", "debug"}
	languages   = []string{"en", "nl"}
	encryptions = []string{EncryptionAuto, EncryptionAge, EncryptionNone}
)

// ValidTag reports whether tag is a valid source tag.
func ValidTag(tag string) bool { return tagRe.MatchString(tag) }

// Validate checks every setting and returns one error per invalid value,
// joined.
func (c *Config) Validate(opts ValidateOptions) error {
	var errs []error
	bad := func(key, format string, args ...any) {
		errs = append(errs, fmt.Errorf("%s: "+format, append([]any{key}, args...)...))
	}

	seen := map[string]string{}
	for i, s := range c.Sources {
		key := fmt.Sprintf("sources[%d]", i)
		if !ValidTag(s.Tag) {
			bad(key+".tag", "%q is not a valid tag (use 1 to 64 letters, digits, - and _)", s.Tag)
		} else if other, dup := seen[strings.ToLower(s.Tag)]; dup {
			bad(key+".tag", "tag %q is used twice (also %q)", s.Tag, other)
		} else {
			seen[strings.ToLower(s.Tag)] = s.Tag
		}
		switch {
		case s.Path == "":
			bad(key+".path", "path is empty")
		case !filepath.IsAbs(s.Path):
			bad(key+".path", "%q is not an absolute path", s.Path)
		case !opts.SkipPathCheck:
			st, err := os.Stat(s.Path)
			if err != nil {
				bad(key+".path", "%q does not exist or cannot be read", s.Path)
			} else if !st.IsDir() {
				bad(key+".path", "%q is not a folder", s.Path)
			}
		}
		checkGlobs(key+".exclude", s.Exclude, bad)
	}
	checkGlobs("exclude", c.Exclude, bad)

	if err := checkRelFolder(c.TargetFolder); err != nil {
		bad("targetFolder", "%v", err)
	}
	if !c.Schedule.Valid() {
		bad("schedule", "%q is not one of insert, day, isoWeek, month", c.Schedule)
	} else if _, err := week.ParsePattern(c.FolderPattern, c.Schedule); err != nil {
		bad("folderPattern", "%v", err)
	}
	if !archiveRe.MatchString(c.ArchiveName) {
		bad("archiveName", "%q must contain {tag} and end in .zip", c.ArchiveName)
	}
	if !c.Rotation.Valid() {
		bad("rotation", "%q is not one of inTurn, any", c.Rotation)
	}
	nonNeg := []struct {
		key string
		v   int
	}{{"minIntervalHours", c.MinIntervalHours}, {"duplicateGuardMinutes", c.DuplicateGuardMinutes}, {"keepPerDisk", c.KeepPerDisk}, {"keepMinDays", c.KeepMinDays}}
	for _, n := range nonNeg {
		if n.v < 0 {
			bad(n.key, "must be 0 or more, not %d", n.v)
		}
	}
	oneOf("freeSpaceAction", c.FreeSpaceAction, []string{FreeSpaceStop, FreeSpaceDeleteOldest}, bad)
	inRange("compressionLevel", c.CompressionLevel, 0, 9, bad)
	for _, e := range c.StoreExtensions {
		if !extRe.MatchString(strings.TrimPrefix(e, ".")) {
			bad("storeExtensions", "%q is not a file extension", e)
		}
	}
	oneOf("lockedFiles", c.LockedFiles, []string{LockedSkip, LockedFail}, bad)
	inRange("minFreePercent", c.MinFreePercent, 0, 90, bad)
	if c.DisksFile == "" {
		bad("disksFile", "must not be empty")
	}
	inRange("waitSeconds", c.WaitSeconds, 0, 3600, bad)
	oneOf("encryption", c.Encryption, encryptions, bad)
	if c.RecipientsFile == "" {
		bad("recipientsFile", "must not be empty")
	}
	inRange("operatorTimeoutMinutes", c.OperatorTimeoutMinutes, 1, 1440, bad)
	oneOf("notify", c.Notify, []string{NotifyAlways, NotifyProblems, NotifyNever}, bad)
	oneOf("language", c.Language, languages, bad)
	if c.WebhookURL != "" {
		u, err := url.Parse(c.WebhookURL)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.User != nil {
			bad("webhookUrl", "%q is not an http(s) URL without credentials", c.WebhookURL)
		}
	}
	inRange("logMaxSizeMB", c.LogMaxSizeMB, 1, 1024, bad)
	inRange("logKeep", c.LogKeep, 0, 100, bad)
	oneOf("logLevel", c.LogLevel, logLevels, bad)
	return errors.Join(errs...)
}

func checkGlobs(key string, globs []string, bad func(string, string, ...any)) {
	for _, g := range globs {
		if g == "" {
			bad(key, "empty pattern")
			continue
		}
		if _, err := path.Match(g, "x"); err != nil {
			bad(key, "%q is not a valid pattern", g)
		}
	}
}

// checkRelFolder checks a relative folder such as "Backup" or
// "Back-ups\Office" on the USB disk.
func checkRelFolder(p string) error {
	norm := strings.ReplaceAll(p, `\`, "/")
	if norm == "" {
		return errors.New("must not be empty")
	}
	if strings.HasPrefix(norm, "/") || filepath.IsAbs(p) || strings.Contains(norm, ":") {
		return fmt.Errorf("%q must be a relative folder on the disk", p)
	}
	for _, comp := range strings.Split(norm, "/") {
		if comp == "." || comp == ".." || !safeCompRe.MatchString(comp) || strings.HasSuffix(comp, ".") || strings.HasSuffix(comp, " ") {
			return fmt.Errorf("%q contains an invalid folder name %q", p, comp)
		}
	}
	return nil
}

func oneOf(key, v string, allowed []string, bad func(string, string, ...any)) {
	for _, a := range allowed {
		if v == a {
			return
		}
	}
	bad(key, "%q is not one of %s", v, strings.Join(allowed, ", "))
}

func inRange(key string, v, lo, hi int, bad func(string, string, ...any)) {
	if v < lo || v > hi {
		bad(key, "must be between %d and %d, not %d", lo, hi, v)
	}
}

// TargetParts returns the target folder as path components.
func (c *Config) TargetParts() []string {
	return strings.Split(strings.ReplaceAll(c.TargetFolder, `\`, "/"), "/")
}

// ArchiveFile returns the archive file name for a tag, e.g. "Data.zip".
func (c *Config) ArchiveFile(tag string) string {
	return strings.ReplaceAll(c.ArchiveName, "{tag}", tag)
}
