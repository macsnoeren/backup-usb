package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"filippo.io/age"

	"github.com/macsnoeren/backup-usb/internal/config"
	"github.com/macsnoeren/backup-usb/internal/crypt"
	"github.com/macsnoeren/backup-usb/internal/fsx"
	"github.com/macsnoeren/backup-usb/internal/rotate"
	"github.com/macsnoeren/backup-usb/internal/verify"
	"github.com/macsnoeren/backup-usb/internal/week"
)

// VerifyOptions are the flags of the verify command.
type VerifyOptions struct {
	Target string
	Week   string
	Tag    string
	All    bool
	// Identities decrypt the archives; empty means no key (only the hash
	// of encrypted files is checked).
	Identities []age.Identity
}

// ArchiveCheck is the result for one archive.
type ArchiveCheck struct {
	Folder string `json:"folder"`
	File   string `json:"file"`
	Tag    string `json:"tag"`
	// Mode is "full" (unpacked completely) or "hash" (only the SHA-256 of
	// the encrypted file, no key given).
	Mode   string `json:"mode"`
	Status string `json:"status"`
	Files  int    `json:"files,omitempty"`
	Bytes  uint64 `json:"bytes,omitempty"`
	Error  string `json:"error,omitempty"`
}

// VerifyReport is the result of the verify command.
type VerifyReport struct {
	Disks    []string       `json:"disks"`
	Archives []ArchiveCheck `json:"archives"`
	Status   string         `json:"status"`
	// Decrypted counts the encrypted archives that were unpacked with a
	// key.
	Decrypted int `json:"decrypted"`
}

var archiveFileRe = regexp.MustCompile(`^([A-Za-z0-9_-]{1,64})[A-Za-z0-9_.-]*\.zip(\.age)?$`)

// connectedDisks returns the recognised disks that are connected (all of
// them, or the one matching target).
func connectedDisks(ctx context.Context, deps *Deps, cfg *config.Config, target string) ([]Candidate, error) {
	reg, err := LoadRegistry(deps.ConfigDir, cfg)
	if err != nil {
		return nil, err
	}
	f, err := scanDisks(ctx, deps, reg, cfg.RequireHardwareSerial, target)
	if err != nil {
		return nil, Errorf(KindIO, err, "err.listDisks")
	}
	if len(f.known) == 0 {
		if len(f.unknown) > 0 {
			return nil, Errorf(KindUnknownDisk, nil, "err.unknownDisk", f.unknown[0])
		}
		return nil, Errorf(KindNoDisk, nil, "err.noDiskAny")
	}
	sort.Slice(f.known, func(i, j int) bool { return f.known[i].Reg.Name < f.known[j].Reg.Name })
	return f.known, nil
}

// periodFolders lists the period folders on a disk, oldest first.
func periodFolders(root *os.Root, cfg *config.Config) ([]rotate.Folder, error) {
	pattern, err := week.ParsePattern(cfg.FolderPattern, cfg.Schedule)
	if err != nil {
		return nil, err
	}
	var patterns []*week.Pattern
	if st, err := ReadDiskState(root); err == nil {
		patterns = st.RotationPatterns(cfg.Schedule)
	}
	return rotate.Find(root, append(patterns, pattern), cfg.Schedule)
}

// selectFolders picks the folders for --week, --all or the newest. The
// default "newest" ignores folders dated in the future.
func selectFolders(folders []rotate.Folder, cfg *config.Config, label string, all bool, now time.Time) ([]rotate.Folder, error) {
	if len(folders) == 0 {
		return nil, Errorf(KindVerify, nil, "err.noBackups")
	}
	switch {
	case all:
		return folders, nil
	case label != "":
		per, err := week.ParseLabel(label, cfg.Schedule)
		if err != nil {
			return nil, Errorf(KindUsage, err, "err.usage", err.Error())
		}
		for _, f := range folders {
			if f.Period.Label() == per.Label() {
				return []rotate.Folder{f}, nil
			}
		}
		return nil, Errorf(KindVerify, nil, "err.noSuchPeriod", per.Label())
	}
	if past := rotate.NotFuture(folders, now); len(past) > 0 {
		return past[len(past)-1:], nil
	}
	return nil, Errorf(KindVerify, nil, "err.noBackups")
}

// archivesIn lists the archives in a period folder.
func archivesIn(root *os.Root, folder, tag string) ([]string, error) {
	f, err := root.Open(folder)
	if err != nil {
		return nil, err
	}
	entries, err := f.ReadDir(-1)
	_ = f.Close()
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		m := archiveFileRe.FindStringSubmatch(e.Name())
		if m == nil || !e.Type().IsRegular() {
			continue
		}
		if tag != "" && !strings.EqualFold(m[1], tag) {
			continue
		}
		out = append(out, e.Name())
	}
	sort.Strings(out)
	return out, nil
}

// checkSHAFile compares a file with its .sha256 file.
func checkSHAFile(ctx context.Context, root *os.Root, rel string) error {
	data, err := readLimited(root, rel+".sha256", 4096)
	if err != nil {
		return fmt.Errorf("hash file missing: %w", err)
	}
	line, _, _ := bytes.Cut(data, []byte("\n"))
	want, name, ok := strings.Cut(strings.TrimPrefix(string(line), `\`), "  ")
	if !ok || name != path.Base(rel) || len(want) != 64 {
		return errors.New("hash file is invalid")
	}
	got, _, err := HashFile(ctx, root, rel)
	if err != nil {
		return err
	}
	if got != strings.ToLower(want) {
		return errors.New("SHA-256 does not match the hash file")
	}
	return nil
}

// Verify checks existing backups again.
func Verify(ctx context.Context, deps *Deps, cfg *config.Config, opts VerifyOptions) (*VerifyReport, error) {
	release, err := deps.AcquireLock()
	if err != nil {
		return nil, err
	}
	defer release()
	disks, err := connectedDisks(ctx, deps, cfg, opts.Target)
	if err != nil {
		return nil, err
	}
	rep := &VerifyReport{Status: StatusOK}
	var firstErr, emptyErr error
	for _, c := range disks {
		rep.Disks = append(rep.Disks, c.Reg.Name)
		root, err := fsx.OpenDiskRoot(c.Vol.Mount, cfg.TargetFolder, false)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				emptyErr = Errorf(KindVerify, nil, "err.noBackups")
				continue
			}
			return rep, Errorf(KindIO, err, "err.read")
		}
		folders, err := periodFolders(root, cfg)
		if err == nil {
			folders, err = selectFolders(folders, cfg, opts.Week, opts.All, deps.Now())
		}
		if err != nil {
			_ = root.Close()
			var e *Error
			if errors.As(err, &e) && (e.Key == "err.noBackups" || e.Key == "err.noSuchPeriod") {
				emptyErr = err
			} else if firstErr == nil {
				firstErr = err
			}
			continue
		}
		for _, f := range folders {
			names, err := archivesIn(root, f.Rel, opts.Tag)
			if err != nil {
				_ = root.Close()
				return rep, Errorf(KindIO, err, "err.read")
			}
			for _, name := range names {
				deps.emit(Event{Event: "state", State: "verifying", Tag: name, Disk: c.Reg.Name})
				chk, err := verifyArchive(ctx, root, f.Rel, name, opts.Identities)
				rep.Archives = append(rep.Archives, chk)
				if err == nil && chk.Mode == "full" && strings.HasSuffix(name, crypt.Suffix) {
					rep.Decrypted++
				}
				if err != nil && firstErr == nil {
					firstErr = err
				}
				deps.Log.Info("verified", "disk", c.Reg.Name, "file", f.Rel+"/"+name, "mode", chk.Mode, "status", chk.Status)
			}
		}
		_ = root.Close()
	}
	if firstErr == nil && len(rep.Archives) == 0 {
		firstErr = emptyErr
		if firstErr == nil {
			firstErr = Errorf(KindVerify, nil, "err.noBackups")
		}
	}
	if firstErr != nil {
		rep.Status = StatusFailed
		return rep, firstErr
	}
	return rep, nil
}

func verifyArchive(ctx context.Context, root *os.Root, folder, name string, ids []age.Identity) (ArchiveCheck, error) {
	rel := path.Join(folder, name)
	m := archiveFileRe.FindStringSubmatch(name)
	chk := ArchiveCheck{Folder: folder, File: name, Tag: m[1], Mode: "full", Status: StatusOK}
	failWith := func(kind Kind, err error) (ArchiveCheck, error) {
		chk.Status = StatusFailed
		chk.Error = err.Error()
		key := "err.verify"
		if kind == KindDecrypt {
			key = "err.decrypt"
		}
		return chk, Errorf(kind, err, key, rel)
	}
	if err := checkSHAFile(ctx, root, rel); err != nil {
		return failWith(KindVerify, err)
	}
	encrypted := strings.HasSuffix(name, crypt.Suffix)
	if encrypted && len(ids) == 0 {
		chk.Mode = "hash"
		return chk, nil
	}
	f, err := fsx.OpenRegular(root, rel)
	if err != nil {
		return failWith(KindIO, err)
	}
	defer func() { _ = f.Close() }() // read-only
	st, err := f.Stat()
	if err != nil {
		return failWith(KindIO, err)
	}
	var rep *verify.Report
	if encrypted {
		ra, size, err := age.DecryptReaderAt(f, st.Size(), ids...)
		if err != nil {
			return failWith(KindDecrypt, err)
		}
		rep, err = verify.Zip(ctx, ra, size, verify.Options{ExpectedFiles: -1})
		if err != nil {
			return failWith(KindVerify, err)
		}
	} else {
		rep, err = verify.Zip(ctx, f, st.Size(), verify.Options{ExpectedFiles: -1})
		if err != nil {
			return failWith(KindVerify, err)
		}
	}
	chk.Files, chk.Bytes = rep.Files, rep.Bytes
	return chk, nil
}

// ParseIdentityFile parses age identities from a key file held in a
// locked buffer (for example the break-glass key).
func ParseIdentityFile(data []byte) ([]age.Identity, []string, error) {
	var ids []age.Identity
	var recipients []string
	// Split lines in place: no copy of the key outside the caller's
	// locked buffer.
	for len(data) > 0 {
		line := data
		if i := bytes.IndexByte(data, '\n'); i >= 0 {
			line, data = data[:i], data[i+1:]
		} else {
			data = nil
		}
		line = bytes.TrimSpace(line)
		if !bytes.HasPrefix(line, []byte("AGE-SECRET-KEY-")) {
			continue
		}
		id, err := age.ParseHybridIdentity(unsafeString(line))
		if err != nil {
			return nil, nil, errors.New("the key file contains an invalid or non post-quantum key")
		}
		ids = append(ids, id)
		recipients = append(recipients, id.Recipient().String())
	}
	if len(ids) == 0 {
		return nil, nil, errors.New("no key found in the key file")
	}
	return ids, recipients, nil
}

// BreakglassTestedFile records when the break-glass key was last used
// successfully. It holds only a time stamp.
const BreakglassTestedFile = "usb-backup.breakglass-tested"

// RecordBreakglassTest writes the time stamp into the log folder (which the
// operator can write without administrator rights).
func RecordBreakglassTest(dir string, cfg *config.Config, now time.Time) error {
	d := cfg.LogDir(dir)
	if err := os.MkdirAll(d, 0o750); err != nil {
		return err
	}
	return fsx.WriteFileAtomic(filepath.Join(d, BreakglassTestedFile), []byte(now.Format(time.RFC3339)+"\n"), 0o644)
}

// BreakglassTested returns when the break-glass key was last tested.
func BreakglassTested(dir string, cfg *config.Config) (time.Time, bool) {
	data, err := config.ReadLimited(filepath.Join(cfg.LogDir(dir), BreakglassTestedFile), 256)
	if err != nil {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(string(data)))
	return t, err == nil
}
