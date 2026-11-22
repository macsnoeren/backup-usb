package app

import (
	"context"

	"filippo.io/age"

	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/macsnoeren/backup-usb/internal/archive"
	"github.com/macsnoeren/backup-usb/internal/config"
	"github.com/macsnoeren/backup-usb/internal/crypt"
	"github.com/macsnoeren/backup-usb/internal/disk"
	"github.com/macsnoeren/backup-usb/internal/fsx"
	"github.com/macsnoeren/backup-usb/internal/logx"
	"github.com/macsnoeren/backup-usb/internal/msg"
	"github.com/macsnoeren/backup-usb/internal/rotate"
	"github.com/macsnoeren/backup-usb/internal/volume"
	"github.com/macsnoeren/backup-usb/internal/week"
)

// Event is a progress event for the GUI and --json-stream.
type Event struct {
	Event string `json:"event"` // state, progress, result, error
	State string `json:"state,omitempty"`
	Tag   string `json:"tag,omitempty"`
	Index int    `json:"index,omitempty"`
	Count int    `json:"count,omitempty"`
	Pct   int    `json:"pct,omitempty"`
	Disk  string `json:"disk,omitempty"`
	// Done and Total are bytes.
	Done  uint64 `json:"done,omitempty"`
	Total uint64 `json:"total,omitempty"`
	// Message is a user-facing text (already translated).
	Message string `json:"message,omitempty"`
	Key     string `json:"key,omitempty"`
}

// Deps are the outside dependencies of the application steps.
type Deps struct {
	Now       func() time.Time
	Finder    volume.Finder
	Log       *logx.Logger
	ConfigDir string
	Lang      msg.Lang
	// Sleep waits between disk searches; tests replace it.
	Sleep func(ctx context.Context, d time.Duration) error
	// Events receives progress; may be nil.
	Events func(Event)
}

func (d *Deps) emit(e Event) {
	if d.Events != nil {
		d.Events(e)
	}
}

func (d *Deps) sleep(ctx context.Context, dur time.Duration) error {
	if d.Sleep != nil {
		return d.Sleep(ctx, dur)
	}
	t := time.NewTimer(dur)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// RunOptions are the flags of the run command.
type RunOptions struct {
	Target  string
	AnyDisk bool
	Tag     string
	Force   bool
	Wait    time.Duration
	DryRun  bool
}

// Candidate is a connected, recognised backup disk.
type Candidate struct {
	Vol volume.Volume
	Reg disk.Registration
}

// found is the result of looking at the connected volumes.
type found struct {
	known   []Candidate
	unknown []string // labels of disks with an ID file that does not match
}

// LoadRegistry reads the registration list from the configuration folder.
func LoadRegistry(dir string, cfg *config.Config) (*disk.Registry, error) {
	data, err := config.ReadLimited(cfg.DisksPath(dir), disk.MaxFileSize)
	if errors.Is(err, fs.ErrNotExist) {
		return &disk.Registry{Version: 1}, nil
	}
	if err != nil {
		return nil, err
	}
	return disk.ParseRegistry(data)
}

// scanDisks looks at all volumes and recognises registered disks. Volumes
// that are not mounted are only mounted when their volume serial belongs to
// a registered disk.
func scanDisks(ctx context.Context, deps *Deps, reg *disk.Registry, requireHW bool, target string) (*found, error) {
	vols, err := deps.Finder.Volumes(ctx)
	if err != nil {
		return nil, fmt.Errorf("list volumes: %w", err)
	}
	f := &found{}
	for _, v := range vols {
		if target != "" && !matchesTarget(v, target) {
			continue
		}
		if v.Mount == "" {
			if !registeredSerial(reg, v.VolumeSerial) {
				continue
			}
			m, _, err := deps.Finder.Mount(ctx, v)
			if err != nil {
				deps.Log.Warn("cannot mount volume", "device", v.Device, "err", err)
				continue
			}
			v = m
		}
		id, _, err := disk.ReadIDFile(v.Mount)
		if errors.Is(err, disk.ErrNoIDFile) {
			if target != "" {
				f.unknown = append(f.unknown, v.Label)
			}
			continue // an ordinary stick: ignore silently
		}
		if err != nil {
			deps.Log.Warn("unreadable ID file", "device", v.Device, "err", err)
			f.unknown = append(f.unknown, v.Label)
			continue
		}
		r, err := disk.Match(v, id, reg, requireHW)
		if err != nil {
			deps.Log.Warn("disk not recognised", "device", v.Device, "label", v.Label, "reason", err)
			f.unknown = append(f.unknown, v.Label)
			continue
		}
		f.known = append(f.known, Candidate{Vol: v, Reg: *r})
	}
	return f, nil
}

func registeredSerial(reg *disk.Registry, serial string) bool {
	for _, d := range reg.Disks {
		if serial != "" && strings.EqualFold(d.VolumeSerial, serial) {
			return true
		}
	}
	return false
}

func matchesTarget(v volume.Volume, target string) bool {
	norm := func(s string) string {
		return strings.TrimRight(strings.ToLower(strings.ReplaceAll(s, `\`, "/")), "/")
	}
	t := norm(target)
	return t != "" && (t == norm(v.Mount) || t == norm(v.Device) || strings.EqualFold(target, v.Label))
}

// slots turns the registry into rotation slots.
func slots(reg *disk.Registry) []week.Slot {
	out := make([]week.Slot, len(reg.Disks))
	for i, d := range reg.Disks {
		out[i] = week.Slot{Name: d.Name, Weeks: d.Weeks, Order: i}
	}
	return out
}

// pickDisk waits for a due disk and returns it.
func pickDisk(ctx context.Context, deps *Deps, cfg *config.Config, reg *disk.Registry, per week.Period, opts RunOptions) (*Candidate, error) {
	due, err := week.Choose(per, cfg.Rotation, slots(reg))
	if err != nil {
		return nil, Errorf(KindUsage, err, "err.config", err.Error())
	}
	deadline := deps.Now().Add(opts.Wait)
	for {
		f, err := scanDisks(ctx, deps, reg, cfg.RequireHardwareSerial, opts.Target)
		if err != nil {
			return nil, Errorf(KindIO, err, "err.listDisks")
		}
		sort.Slice(f.known, func(i, j int) bool { return f.known[i].Reg.Name < f.known[j].Reg.Name })
		for i := range f.known {
			c := &f.known[i]
			if opts.AnyDisk || opts.Target != "" || containsFold(due, c.Reg.Name) {
				return c, nil
			}
		}
		if deps.Now().Before(deadline) {
			if err := deps.sleep(ctx, time.Second); err != nil {
				return nil, Errorf(KindIO, err, "err.cancelled")
			}
			continue
		}
		switch {
		case len(f.known) > 0:
			return nil, Errorf(KindWrongDisk, nil, "err.wrongDisk", per.Label(), strings.Join(dueOrAny(due), " / "), f.known[0].Reg.Name)
		case len(f.unknown) > 0:
			return nil, Errorf(KindUnknownDisk, nil, "err.unknownDisk", f.unknown[0])
		default:
			return nil, Errorf(KindNoDisk, nil, "err.noDisk", strings.Join(dueOrAny(due), " / "))
		}
	}
}

func dueOrAny(due []string) []string {
	if len(due) == 0 {
		return []string{"?"}
	}
	return due
}

func containsFold(list []string, s string) bool {
	for _, x := range list {
		if strings.EqualFold(x, s) {
			return true
		}
	}
	return false
}

// checkDuplicate refuses a run shortly after the previous one.
func checkDuplicate(lb *LastBackup, now time.Time, folder string, cfg *config.Config) error {
	if lb == nil {
		return nil
	}
	since := now.Sub(lb.Time)
	if since >= 0 && since < time.Duration(cfg.DuplicateGuardMinutes)*time.Minute {
		return Errorf(KindSkipped, nil, "err.duplicate", cfg.DuplicateGuardMinutes)
	}
	if lb.Folder == folder && since >= 0 && since < time.Duration(cfg.MinIntervalHours)*time.Hour {
		return Errorf(KindSkipped, nil, "err.recent", cfg.MinIntervalHours)
	}
	return nil
}

// sourcesFor returns the configured sources, or just the one with tag.
func sourcesFor(cfg *config.Config, tag string) ([]config.Source, error) {
	if len(cfg.Sources) == 0 {
		return nil, Errorf(KindUsage, nil, "err.noSources")
	}
	if tag == "" {
		return cfg.Sources, nil
	}
	for _, s := range cfg.Sources {
		if strings.EqualFold(s.Tag, tag) {
			return []config.Source{s}, nil
		}
	}
	return nil, Errorf(KindUsage, nil, "err.unknownTag", tag)
}

func archiveOptions(cfg *config.Config, s config.Source) archive.Options {
	return archive.Options{
		CompressionLevel: cfg.CompressionLevel,
		StoreExtensions:  cfg.StoreExtensions,
		Exclude:          append(append([]string(nil), cfg.Exclude...), s.Exclude...),
		LockedFiles:      cfg.LockedFiles,
		FollowSymlinks:   cfg.FollowSymlinks,
	}
}

// Run makes a backup of all source folders, following the steps of the
// flow chart in the design. It returns the report and an *Error whose kind
// determines the exit code. The report is nil only for errors before a disk
// was chosen.
func Run(ctx context.Context, deps *Deps, cfg *config.Config, opts RunOptions) (*RunReport, error) {
	p := msg.Printer{Lang: deps.Lang}
	now := deps.Now()
	rep := &RunReport{Status: StatusFailed, Started: now}
	per := week.For(now, cfg.Schedule)
	pattern, err := week.ParsePattern(cfg.FolderPattern, cfg.Schedule)
	if err != nil {
		return nil, Errorf(KindUsage, err, "err.config", err.Error())
	}
	folder := pattern.Folder(per)
	rep.Period, rep.Folder = per.Label(), folder

	sources, err := sourcesFor(cfg, opts.Tag)
	if err != nil {
		return nil, err
	}
	reg, err := LoadRegistry(deps.ConfigDir, cfg)
	if err != nil {
		return nil, Errorf(KindTampered, err, "err.registry")
	}

	deps.emit(Event{Event: "state", State: "searching"})
	c, err := pickDisk(ctx, deps, cfg, reg, per, opts)
	if err != nil {
		deps.Log.Info("no backup", "reason", err)
		return nil, err
	}
	rep.Disk, rep.Device, rep.Mount = c.Reg.Name, c.Vol.Device, c.Vol.Mount
	deps.Log.Info("backup disk found", "disk", c.Reg.Name, "device", c.Vol.Device, "period", per.Label())

	root, err := fsx.OpenDiskRoot(c.Vol.Mount, cfg.TargetFolder, !opts.DryRun)
	if opts.DryRun && errors.Is(err, fs.ErrNotExist) {
		// Nothing on the disk yet; a dry run must not create anything.
		root, err = emptyRoot()
	}
	if err != nil {
		return rep, Errorf(KindDiskCheck, err, "err.notWritable")
	}
	defer func() { _ = root.Close() }()

	if !opts.Force {
		if err := checkDuplicate(ReadLastBackup(root), now, folder, cfg); err != nil {
			rep.Status = StatusSkipped
			return rep, err
		}
	}

	// Encryption: on when the recipients file exists (auto) or required
	// (age). Never write unencrypted data when it should be encrypted.
	enc, err := encryptionFor(deps.ConfigDir, cfg)
	if err != nil {
		return rep, err
	}
	if enc != nil {
		rep.Encrypted = true
		rep.Keys = enc.Fingerprints()
	}

	// Disk checks.
	need := SpaceNeed{MinFreePercent: cfg.MinFreePercent}
	var sizes []uint64
	for _, s := range sources {
		st, err := os.Stat(s.Path)
		if err != nil || !st.IsDir() {
			return rep, Errorf(KindIO, err, "err.sourceMissing", s.Tag, s.Path)
		}
		_, b, err := archive.Scan(s.Path, archiveOptions(cfg, s))
		if err != nil {
			return rep, Errorf(KindIO, err, "err.read")
		}
		sizes = append(sizes, b)
	}
	need.SourceBytes = sizes
	health := deps.Finder.Health(ctx, c.Vol)
	rep.Health = string(health)

	state, err := ReadDiskState(root)
	if err != nil {
		deps.Log.Warn("disk state unreadable, starting a new one", "err", err)
		state = &DiskState{Version: 1, Folders: map[string]FolderRecord{}}
	}
	patterns := append(state.RotationPatterns(cfg.Schedule), pattern)
	pol := rotate.Policy{KeepPerDisk: cfg.KeepPerDisk, KeepMinDays: cfg.KeepMinDays}

	if opts.DryRun {
		return dryRun(ctx, deps, rep, c, root, patterns, cfg, pol, need, folder, now)
	}

	if err := CheckFileSystem(c.Vol, need); err != nil {
		return rep, err
	}
	free, _, err := deps.Finder.Space(ctx, c.Vol)
	if err != nil {
		return rep, Errorf(KindDiskCheck, err, "err.spaceUnknown")
	}
	if err := CheckSpace(free, need); err != nil {
		if cfg.FreeSpaceAction != config.FreeSpaceDeleteOldest {
			return rep, err
		}
		freed, ferr := freeSpace(ctx, deps, c, root, patterns, cfg, pol, need, folder, now)
		rep.Rotated = append(rep.Rotated, freed...)
		if ferr != nil {
			return rep, ferr
		}
	}
	if err := CheckWritable(root.Name()); err != nil {
		return rep, err
	}
	if removed, err := CleanupParts(root.Name()); err != nil {
		deps.Log.Warn("cleanup of part files failed", "err", err)
	} else if len(removed) > 0 {
		deps.Log.Info("removed part files of an interrupted run", "count", len(removed))
	}

	if err := root.MkdirAll(folder, 0o750); err != nil {
		return rep, Errorf(KindIO, err, "err.write")
	}

	// One zip per source.
	var firstErr error
	warnings := false
	for i, s := range sources {
		deps.emit(Event{Event: "state", State: "backup", Tag: s.Tag, Index: i + 1, Count: len(sources), Disk: c.Reg.Name})
		sr := SourceReport{Tag: s.Tag, Archive: archiveName(cfg, s.Tag, enc != nil)}
		files, _, _ := archive.Scan(s.Path, archiveOptions(cfg, s))
		if files == 0 && isEmptyDir(s.Path) {
			sr.Status = StatusEmpty
			warnings = true
			rep.Sources = append(rep.Sources, sr)
			deps.Log.Warn("source folder is empty, no zip", "tag", s.Tag)
			continue
		}
		aopts := archiveOptions(cfg, s)
		aopts.ManifestTime = now
		total := sizes[i]
		aopts.Progress = func(done uint64, _ string) {
			pct := 100
			if total > 0 {
				pct = int(min(done*100/total, 100))
			}
			deps.emit(Event{Event: "progress", Tag: s.Tag, Pct: pct, Done: done, Total: total})
		}
		job := ZipJob{Src: s.Path, Root: root, Rel: path.Join(folder, sr.Archive), Opts: aopts, Verify: cfg.VerifyAfterBackup}
		var out *ZipOutcome
		if enc != nil {
			out, err = WriteVerifiedEncrypted(ctx, EncJob{ZipJob: job, Recipients: enc.age})
		} else {
			out, err = WriteVerifiedZip(ctx, job)
		}
		if err != nil {
			sr.Status = StatusFailed
			sr.Error = errorText(p, err)
			rep.Sources = append(rep.Sources, sr)
			deps.Log.Error("source failed", "tag", s.Tag, "err", err)
			if firstErr == nil {
				firstErr = err
			}
			if errors.Is(err, context.Canceled) || ctx.Err() != nil {
				break
			}
			continue
		}
		fillSource(&sr, out, cfg.VerifyAfterBackup || enc != nil)
		var rcpts []age.Recipient
		if enc != nil {
			rcpts = enc.age
		}
		if err := WriteIndex(root, folder, s.Tag, per.Label(), sr.Archive, now, out.Result, rcpts); err != nil {
			deps.Log.Warn("index not written", "tag", s.Tag, "err", err)
			sr.Status = StatusWarning
			sr.WarningCount++
			sr.Warnings = append(sr.Warnings, archive.Skipped{Path: IndexName(s.Tag, enc != nil), Reason: "index not written; restore reads the whole archive"})
		}
		if sr.SkippedCount > 0 || sr.WarningCount > 0 {
			warnings = true
		}
		rep.Sources = append(rep.Sources, sr)
		deps.Log.Info("source done", "tag", s.Tag, "files", sr.Files, "bytes", sr.Bytes, "zip", sr.ZipBytes)
	}

	rep.Finished = deps.Now()
	if firstErr != nil {
		rep.Status = StatusFailed
		rep.Error = errorText(p, firstErr)
		rep.Messages = append(rep.Messages, p.T("msg.notRotated"))
		rep.MessageKeys = append(rep.MessageKeys, "msg.notRotated")
		writeReportFile(deps, root, folder, rep, cfg)
		return rep, firstErr
	}

	// Everything verified: record state, then rotate.
	state.AddPattern(cfg.FolderPattern, cfg.Schedule)
	state.Folders[folder] = FolderRecord{Time: now, Keys: rep.Keys}
	folders, err := rotate.Find(root, patterns, cfg.Schedule)
	if err != nil {
		deps.Log.Warn("cannot list folders for rotation", "err", err)
	} else {
		del := rotate.Plan(folders, pol, now, folder)
		removed, err := rotate.Delete(root, del)
		for _, r := range removed {
			delete(state.Folders, r)
		}
		rep.Rotated = append(rep.Rotated, removed...)
		if err != nil {
			deps.Log.Error("rotation failed", "err", err)
		}
		if len(removed) > 0 {
			deps.Log.Info("rotated old folders", "folders", strings.Join(removed, ","))
		}
		rep.FolderCount = len(folders) - len(removed)
	}
	if err := state.Write(root); err != nil {
		deps.Log.Warn("cannot write disk state", "err", err)
	}
	if err := WriteLastBackup(root, LastBackup{Time: now, Folder: folder}); err != nil {
		deps.Log.Warn("cannot write last backup time", "err", err)
	}
	if free, _, err := deps.Finder.Space(ctx, c.Vol); err == nil {
		rep.FreeBytes = free
	}
	switch health {
	case volume.Warning:
		warnings = true
		rep.addMessage(p, "msg.healthWarning")
	case volume.Unhealthy:
		warnings = true
		rep.addMessage(p, "msg.healthUnhealthy")
	}
	rep.Status = StatusOK
	if warnings {
		rep.Status = StatusWarning
	}
	rep.addMessage(p, "msg.canRemove")
	rep.Finished = deps.Now()
	writeReportFile(deps, root, folder, rep, cfg)
	deps.emit(Event{Event: "state", State: "done", Disk: c.Reg.Name})
	if cfg.EjectAfter {
		if err := deps.Finder.Eject(ctx, c.Vol); err != nil {
			deps.Log.Warn("eject failed", "err", err)
		}
	}
	if warnings {
		return rep, Errorf(KindWarning, nil, "msg.warnings")
	}
	return rep, nil
}

func (r *RunReport) addMessage(p msg.Printer, key string, args ...any) {
	r.Messages = append(r.Messages, p.T(key, args...))
	r.MessageKeys = append(r.MessageKeys, key)
}

// Redact removes file names from the report; used when the backup is
// encrypted, because report.txt stays readable on the disk.
func (r *RunReport) Redact() {
	for i := range r.Sources {
		r.Sources[i].Skipped = nil
		r.Sources[i].Warnings = nil
	}
}

func fillSource(sr *SourceReport, out *ZipOutcome, verified bool) {
	res := out.Result
	sr.Status = StatusOK
	sr.Files, sr.Dirs, sr.Bytes = res.Files, res.Dirs, res.Bytes
	sr.ZipBytes, sr.SHA256, sr.Verified = out.Size, out.SHA256, verified
	sr.SkippedCount, sr.Skipped = len(res.Skipped), res.Skipped
	sr.WarningCount, sr.Warnings = len(res.Warnings), res.Warnings
	if sr.SkippedCount > 0 || sr.WarningCount > 0 {
		sr.Status = StatusWarning
	}
}

func isEmptyDir(p string) bool {
	entries, err := os.ReadDir(p)
	return err == nil && len(entries) == 0
}

// errorText returns the user-facing text of an error.
func errorText(p msg.Printer, err error) string {
	var e *Error
	if errors.As(err, &e) && msg.Has(e.Key) {
		return p.T(e.Key, e.Args...)
	}
	return err.Error()
}

// freeSpace deletes the oldest folders until the backup fits
// (freeSpaceAction: deleteOldest), never below keepMinDays and never the
// current folder.
func freeSpace(ctx context.Context, deps *Deps, c *Candidate, root *os.Root, patterns []*week.Pattern, cfg *config.Config, pol rotate.Policy, need SpaceNeed, current string, now time.Time) ([]string, error) {
	var removed []string
	for {
		folders, err := rotate.Find(root, patterns, cfg.Schedule)
		if err != nil {
			return removed, Errorf(KindIO, err, "err.read")
		}
		var victim *rotate.Folder
		for i := range folders {
			f := folders[i]
			if f.Rel != current && !rotate.TooYoung(f, pol, now) {
				victim = &f
				break
			}
		}
		if victim == nil {
			free, _, _ := deps.Finder.Space(ctx, c.Vol)
			return removed, Errorf(KindDiskCheck, nil, "err.space", humanBytes(need.Required()), humanBytes(free))
		}
		r, err := rotate.Delete(root, []rotate.Folder{*victim})
		removed = append(removed, r...)
		if err != nil {
			return removed, Errorf(KindIO, err, "err.write")
		}
		deps.Log.Info("deleted oldest folder to free space", "folder", victim.Rel)
		free, _, err := deps.Finder.Space(ctx, c.Vol)
		if err != nil {
			return removed, Errorf(KindDiskCheck, err, "err.spaceUnknown")
		}
		if CheckSpace(free, need) == nil {
			return removed, nil
		}
	}
}

// dryRun reports what a run would do without writing anything.
func dryRun(ctx context.Context, deps *Deps, rep *RunReport, c *Candidate, root *os.Root, patterns []*week.Pattern, cfg *config.Config, pol rotate.Policy, need SpaceNeed, folder string, now time.Time) (*RunReport, error) {
	var total uint64
	for _, b := range need.SourceBytes {
		total += b
	}
	info := &DryRunInfo{ExpectedBytes: total, RequiredBytes: need.Required(), WouldRotate: []string{}}
	if folders, err := rotate.Find(root, patterns, cfg.Schedule); err == nil {
		// The new folder counts if it does not exist yet.
		exists := false
		for _, f := range folders {
			exists = exists || f.Rel == folder
		}
		if !exists {
			if per, ok := patterns[len(patterns)-1].Match(folder, cfg.Schedule); ok {
				folders = append(folders, rotate.Folder{Rel: folder, Period: per})
			}
		}
		for _, f := range rotate.Plan(folders, pol, now, folder) {
			info.WouldRotate = append(info.WouldRotate, f.Rel)
		}
	}
	if free, _, err := deps.Finder.Space(ctx, c.Vol); err == nil {
		rep.FreeBytes = free
	}
	for _, s := range cfg.Sources {
		rep.Sources = append(rep.Sources, SourceReport{Tag: s.Tag, Status: StatusDryRun, Archive: archiveName(cfg, s.Tag, rep.Encrypted)})
	}
	rep.DryRun = info
	rep.Status = StatusDryRun
	rep.Finished = deps.Now()
	return rep, nil
}

// writeReportFile writes report.txt into the period folder.
func writeReportFile(deps *Deps, root *os.Root, folder string, rep *RunReport, cfg *config.Config) {
	if rep.Encrypted {
		rep.Redact()
	}
	if err := fsx.RootWriteFileAtomic(root, path.Join(folder, ReportFile), []byte(RenderReport(rep, deps.Lang)), 0o644); err != nil {
		deps.Log.Warn("cannot write report", "err", err)
	}
	_ = cfg
}

// emptyRoot is a read-only stand-in for a backup folder that does not exist
// yet (dry run).
func emptyRoot() (*os.Root, error) {
	dir, err := os.MkdirTemp("", "usb-backup-dry-run")
	if err != nil {
		return nil, err
	}
	r, err := os.OpenRoot(dir)
	_ = os.Remove(dir)
	return r, err
}

// encryption holds the recipients of an encrypted run.
type encryption struct {
	*Recipients
	age []age.Recipient
}

// encryptionFor decides whether this run encrypts. It returns nil for an
// unencrypted run, and an error (KindDiskCheck, exit code 10) when
// encryption is required but the recipients are missing or invalid.
func encryptionFor(dir string, cfg *config.Config) (*encryption, error) {
	if cfg.Encryption == config.EncryptionNone {
		return nil, nil
	}
	p := cfg.RecipientsPath(dir)
	rec, err := ReadRecipients(p)
	if errors.Is(err, fs.ErrNotExist) {
		if cfg.Encryption == config.EncryptionAge {
			return nil, Errorf(KindDiskCheck, nil, "err.noRecipients", p)
		}
		return nil, nil
	}
	if err != nil {
		return nil, Errorf(KindDiskCheck, err, "err.badRecipients", p)
	}
	rs, err := rec.AgeRecipients()
	if err != nil {
		return nil, Errorf(KindDiskCheck, err, "err.badRecipients", p)
	}
	return &encryption{Recipients: rec, age: rs}, nil
}

// archiveName is the file name of a source's archive.
func archiveName(cfg *config.Config, tag string, encrypted bool) string {
	n := cfg.ArchiveFile(tag)
	if encrypted {
		n += crypt.Suffix
	}
	return n
}
