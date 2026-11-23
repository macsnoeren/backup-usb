package app

import (
	"context"
	"errors"
	"io/fs"
	"time"

	"github.com/macsnoeren/backup-usb/internal/config"
	"github.com/macsnoeren/backup-usb/internal/fsx"
	"github.com/macsnoeren/backup-usb/internal/rotate"
	"github.com/macsnoeren/backup-usb/internal/week"
)

// DiskStatus is one connected backup disk.
type DiskStatus struct {
	Name       string     `json:"name"`
	Device     string     `json:"device"`
	Mount      string     `json:"mount"`
	Health     string     `json:"health"`
	FreeBytes  uint64     `json:"freeBytes"`
	TotalBytes uint64     `json:"totalBytes"`
	Folders    int        `json:"folders"`
	LastBackup *time.Time `json:"lastBackup,omitempty"`
	Due        bool       `json:"due"`
	// NeedsUpdate is set when the disk holds backups for a retired key
	// (see keys rewrap).
	NeedsUpdate bool `json:"needsUpdate"`
}

// StatusReport is what status shows.
type StatusReport struct {
	Period    string       `json:"period"`
	Due       []string     `json:"due"`
	Encrypted bool         `json:"encrypted"`
	Disks     []DiskStatus `json:"disks"`
	// BreakglassTested is when the break-glass key last worked; nil if it
	// was never tested.
	BreakglassTested *time.Time `json:"breakglassTested,omitempty"`
}

// Status describes the connected backup disks. It needs no password.
func Status(ctx context.Context, deps *Deps, cfg *config.Config) (*StatusReport, error) {
	reg, err := LoadRegistry(deps.ConfigDir, cfg)
	if err != nil {
		return nil, err
	}
	now := deps.Now()
	per := week.For(now, cfg.Schedule)
	due, _ := week.Choose(per, cfg.Rotation, slots(reg))
	rep := &StatusReport{Period: per.Label(), Due: due, Disks: []DiskStatus{}}
	if due == nil {
		rep.Due = []string{}
	}
	if t, ok := BreakglassTested(deps.ConfigDir); ok {
		rep.BreakglassTested = &t
	}
	if enc, err := encryptionFor(deps.ConfigDir, cfg); err == nil && enc != nil {
		rep.Encrypted = true
	}
	f, err := scanDisks(ctx, deps, reg, cfg.RequireHardwareSerial, "")
	if err != nil {
		return nil, Errorf(KindIO, err, "err.listDisks")
	}
	current := currentFingerprints(deps.ConfigDir, cfg)
	for _, c := range f.known {
		ds := DiskStatus{Name: c.Reg.Name, Device: c.Vol.Device, Mount: c.Vol.Mount, Health: string(deps.Finder.Health(ctx, c.Vol)), Due: containsFold(due, c.Reg.Name)}
		ds.FreeBytes, ds.TotalBytes, _ = deps.Finder.Space(ctx, c.Vol)
		root, err := fsx.OpenDiskRoot(c.Vol.Mount, cfg.TargetFolder, false)
		if err == nil {
			if folders, err := periodFolders(root, cfg); err == nil {
				ds.Folders = len(rotate.NotFuture(folders, now))
			}
			if lb := ReadLastBackup(root); lb != nil {
				t := lb.Time
				ds.LastBackup = &t
			}
			if st, err := ReadDiskState(root); err == nil {
				ds.NeedsUpdate = needsUpdate(st, current)
			}
			_ = root.Close()
		} else if !errors.Is(err, fs.ErrNotExist) {
			deps.Log.Warn("cannot open backup folder", "disk", c.Reg.Name, "err", err)
		}
		rep.Disks = append(rep.Disks, ds)
	}
	return rep, nil
}

// currentFingerprints are the fingerprints in the recipients file now.
func currentFingerprints(dir string, cfg *config.Config) map[string]bool {
	out := map[string]bool{}
	if rec, err := ReadRecipients(cfg.RecipientsPath(dir)); err == nil {
		for _, fp := range rec.Fingerprints() {
			out[fp] = true
		}
	}
	return out
}

// needsUpdate reports whether a disk has folders encrypted for a key that
// is no longer in the recipients file.
func needsUpdate(st *DiskState, current map[string]bool) bool {
	if len(current) == 0 {
		return false
	}
	for _, f := range st.Folders {
		for _, k := range f.Keys {
			if !current[k] {
				return true
			}
		}
	}
	return false
}
