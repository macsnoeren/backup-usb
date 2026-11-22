package app

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"filippo.io/age"

	"github.com/macsnoeren/backup-usb/internal/archive"
	"github.com/macsnoeren/backup-usb/internal/config"
	"github.com/macsnoeren/backup-usb/internal/fsx"
	"github.com/macsnoeren/backup-usb/internal/rotate"
)

// ListOptions are the flags of the list command.
type ListOptions struct {
	Target     string
	Week       string
	Tag        string
	Path       string
	Search     string
	Identities []age.Identity
}

// ListPeriod is one period folder with its sources.
type ListPeriod struct {
	Disk      string   `json:"disk"`
	Period    string   `json:"period"`
	Folder    string   `json:"folder"`
	Tags      []string `json:"tags"`
	Encrypted bool     `json:"encrypted"`
}

// ListEntry is one file or folder in a backup.
type ListEntry struct {
	Path     string    `json:"path"`
	Dir      bool      `json:"dir,omitempty"`
	Size     uint64    `json:"size"`
	Modified time.Time `json:"modified"`
}

// ListResult is what list shows: periods, or the entries of one source.
type ListResult struct {
	Periods []ListPeriod `json:"periods,omitempty"`
	Disk    string       `json:"disk,omitempty"`
	Period  string       `json:"period,omitempty"`
	Tag     string       `json:"tag,omitempty"`
	Entries []ListEntry  `json:"entries,omitempty"`
	// FromIndex is false when the index was missing and the archive
	// itself was read (slower).
	FromIndex bool `json:"fromIndex"`
}

// openedDisk is a connected disk with its backup folder opened.
type openedDisk struct {
	Candidate
	root    *os.Root
	folders []rotate.Folder
}

func openDisks(ctx context.Context, deps *Deps, cfg *config.Config, target string) ([]openedDisk, error) {
	disks, err := connectedDisks(ctx, deps, cfg, target)
	if err != nil {
		return nil, err
	}
	var out []openedDisk
	for _, c := range disks {
		root, err := fsx.OpenDiskRoot(c.Vol.Mount, cfg.TargetFolder, false)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			closeDisks(out)
			return nil, Errorf(KindIO, err, "err.read")
		}
		folders, err := periodFolders(root, cfg)
		if err != nil {
			_ = root.Close()
			closeDisks(out)
			return nil, Errorf(KindIO, err, "err.read")
		}
		out = append(out, openedDisk{Candidate: c, root: root, folders: folders})
	}
	if len(out) == 0 {
		return nil, Errorf(KindVerify, nil, "err.noBackups")
	}
	return out, nil
}

func closeDisks(ds []openedDisk) {
	for _, d := range ds {
		_ = d.root.Close()
	}
}

// findPeriod returns the disk and folder holding the period label (the
// newest one when label is empty).
func findPeriod(disks []openedDisk, cfg *config.Config, label string) (*openedDisk, rotate.Folder, error) {
	var best *openedDisk
	var bestF rotate.Folder
	for i := range disks {
		sel, err := selectFolders(disks[i].folders, cfg, label, false)
		if err != nil {
			continue
		}
		f := sel[0]
		if best == nil || f.Period.Start.After(bestF.Period.Start) {
			best, bestF = &disks[i], f
		}
	}
	if best == nil {
		if label != "" {
			return nil, rotate.Folder{}, Errorf(KindVerify, nil, "err.noSuchPeriod", label)
		}
		return nil, rotate.Folder{}, Errorf(KindVerify, nil, "err.noBackups")
	}
	return best, bestF, nil
}

// List shows the periods, or the files of one source from its index.
func List(ctx context.Context, deps *Deps, cfg *config.Config, opts ListOptions) (*ListResult, error) {
	disks, err := openDisks(ctx, deps, cfg, opts.Target)
	if err != nil {
		return nil, err
	}
	defer closeDisks(disks)
	if opts.Tag == "" {
		res := &ListResult{}
		for _, d := range disks {
			for _, f := range d.folders {
				if opts.Week != "" {
					if sel, err := selectFolders([]rotate.Folder{f}, cfg, opts.Week, false); err != nil || len(sel) == 0 {
						continue
					}
				}
				names, err := archivesIn(d.root, f.Rel, "")
				if err != nil {
					return nil, Errorf(KindIO, err, "err.read")
				}
				p := ListPeriod{Disk: d.Reg.Name, Period: f.Period.Label(), Folder: f.Rel, Tags: []string{}}
				for _, n := range names {
					m := archiveFileRe.FindStringSubmatch(n)
					p.Tags = append(p.Tags, m[1])
					p.Encrypted = p.Encrypted || m[2] != ""
				}
				res.Periods = append(res.Periods, p)
			}
		}
		if opts.Week != "" && len(res.Periods) == 0 {
			return nil, Errorf(KindVerify, nil, "err.noSuchPeriod", opts.Week)
		}
		return res, nil
	}
	d, f, err := findPeriod(disks, cfg, opts.Week)
	if err != nil {
		return nil, err
	}
	src, err := openSource(d.root, f.Rel, opts.Tag, opts.Identities)
	if err != nil {
		return nil, err
	}
	defer src.Close()
	res := &ListResult{Disk: d.Reg.Name, Period: f.Period.Label(), Tag: opts.Tag, FromIndex: src.fromIndex}
	res.Entries = filterEntries(src.index.Entries, opts.Path, opts.Search)
	return res, nil
}

// filterEntries returns the direct children of dir, or with search every
// entry whose path contains it (case-insensitive).
func filterEntries(entries []archive.Entry, dir, search string) []ListEntry {
	dir = strings.Trim(strings.ReplaceAll(dir, `\`, "/"), "/")
	search = strings.ToLower(search)
	seen := map[string]bool{}
	var out []ListEntry
	for _, e := range entries {
		p := strings.TrimSuffix(e.Path, "/")
		if search != "" {
			if strings.Contains(strings.ToLower(p), search) && (dir == "" || strings.HasPrefix(p, dir+"/")) {
				out = append(out, ListEntry{Path: p, Dir: e.Dir, Size: e.Size, Modified: e.Modified})
			}
			continue
		}
		rest := p
		if dir != "" {
			var ok bool
			if rest, ok = strings.CutPrefix(p, dir+"/"); !ok {
				continue
			}
		}
		child, _, deeper := strings.Cut(rest, "/")
		full := path.Join(dir, child)
		if seen[full] {
			continue
		}
		seen[full] = true
		if deeper {
			out = append(out, ListEntry{Path: full, Dir: true})
			continue
		}
		out = append(out, ListEntry{Path: full, Dir: e.Dir, Size: e.Size, Modified: e.Modified})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Dir != out[j].Dir {
			return out[i].Dir
		}
		return out[i].Path < out[j].Path
	})
	return out
}
