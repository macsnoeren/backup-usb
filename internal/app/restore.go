package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"filippo.io/age"

	"github.com/macsnoeren/backup-usb/internal/archive"
	"github.com/macsnoeren/backup-usb/internal/config"
	"github.com/macsnoeren/backup-usb/internal/fsx"
)

// RestoreOptions are the flags of the restore command.
type RestoreOptions struct {
	Target string
	Week   string
	Tag    string
	// Files are paths in the backup (a file or a folder); empty restores
	// the whole source.
	Files []string
	// To is the destination folder; Original restores to the source folder
	// of the tag instead.
	To         string
	Original   bool
	Identities []age.Identity
}

// Renamed is a restored file that got a new name because the original name
// was taken.
type Renamed struct {
	Path string `json:"path"`
	As   string `json:"as"`
}

// RestoreReport is the result of a restore.
type RestoreReport struct {
	Disk        string            `json:"disk"`
	Period      string            `json:"period"`
	Tag         string            `json:"tag"`
	Destination string            `json:"destination"`
	Files       int               `json:"files"`
	Dirs        int               `json:"dirs"`
	Bytes       uint64            `json:"bytes"`
	Renamed     []Renamed         `json:"renamed,omitempty"`
	Skipped     []archive.Skipped `json:"skipped,omitempty"`
	FromIndex   bool              `json:"fromIndex"`
	// Decrypted is set when the archive was encrypted.
	Decrypted bool `json:"decrypted"`
}

// Restore decrypts and unpacks files of one backup. Existing files are never
// overwritten: a restored file whose name is taken becomes
// "name (restored 2026-11-22).ext". Every file is checked against the
// SHA-256 from the index (or the manifest) before it gets its name.
func Restore(ctx context.Context, deps *Deps, cfg *config.Config, opts RestoreOptions) (*RestoreReport, error) {
	if opts.Tag == "" {
		return nil, Errorf(KindUsage, nil, "err.restoreTag")
	}
	dest := opts.To
	if opts.Original {
		src, err := sourcesFor(cfg, opts.Tag)
		if err != nil {
			return nil, err
		}
		dest = src[0].Path
	}
	if dest == "" {
		return nil, Errorf(KindUsage, nil, "err.restoreDest")
	}
	release, err := deps.AcquireLock()
	if err != nil {
		return nil, err
	}
	defer release()
	disks, err := openDisks(ctx, deps, cfg, opts.Target)
	if err != nil {
		return nil, err
	}
	defer closeDisks(disks)
	d, f, err := findPeriod(disks, cfg, opts.Week, deps.Now())
	if err != nil {
		return nil, err
	}
	src, err := openSource(d.root, f.Rel, opts.Tag, opts.Identities)
	if err != nil {
		return nil, err
	}
	defer src.Close()
	entries, err := selectEntries(src.index.Entries, opts.Files)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dest, 0o750); err != nil {
		return nil, Errorf(KindIO, err, "err.restoreWrite", dest)
	}
	root, err := os.OpenRoot(dest)
	if err != nil {
		return nil, Errorf(KindIO, err, "err.restoreWrite", dest)
	}
	defer func() { _ = root.Close() }()
	rep := &RestoreReport{Disk: d.Reg.Name, Period: f.Period.Label(), Tag: src.index.Tag, Destination: dest, FromIndex: src.fromIndex, Decrypted: src.encrypted}
	day := deps.Now().Format("2006-01-02")
	var dirTimes []archive.Entry
	for i, e := range entries {
		if err := ctx.Err(); err != nil {
			return rep, Errorf(KindIO, err, "err.cancelled")
		}
		deps.emit(Event{Event: "progress", Tag: rep.Tag, Pct: (i + 1) * 100 / len(entries)})
		rel, err := fsx.SafeRel(e.Path)
		if err != nil {
			rep.Skipped = append(rep.Skipped, archive.Skipped{Path: e.Path, Reason: "name cannot be restored on this system"})
			continue
		}
		if e.Dir {
			if err := root.MkdirAll(rel, 0o750); err != nil {
				return rep, Errorf(KindIO, err, "err.restoreWrite", rel)
			}
			rep.Dirs++
			dirTimes = append(dirTimes, e)
			continue
		}
		if dir := path.Dir(rel); dir != "." {
			if err := root.MkdirAll(dir, 0o750); err != nil {
				return rep, Errorf(KindIO, err, "err.restoreWrite", rel)
			}
		}
		if e.Link != "" {
			if reason := restoreLink(root, rel, e, src); reason != "" {
				rep.Skipped = append(rep.Skipped, archive.Skipped{Path: e.Path, Reason: reason})
			} else {
				rep.Files++
			}
			continue
		}
		final, err := freeName(root, rel, day)
		if err != nil {
			return rep, Errorf(KindIO, err, "err.restoreWrite", rel)
		}
		if err := restoreFile(ctx, root, src, e, final); err != nil {
			return rep, err
		}
		if final != rel {
			rep.Renamed = append(rep.Renamed, Renamed{Path: rel, As: final})
		}
		rep.Files++
		rep.Bytes += e.Size
	}
	// Folder times last, after their contents were written.
	for i := len(dirTimes) - 1; i >= 0; i-- {
		e := dirTimes[i]
		if rel, err := fsx.SafeRel(e.Path); err == nil && !e.Modified.IsZero() {
			_ = root.Chtimes(rel, e.Modified, e.Modified)
		}
	}
	deps.Log.Info("restored", "disk", rep.Disk, "period", rep.Period, "tag", rep.Tag, "files", rep.Files)
	return rep, nil
}

// selectEntries returns the entries for the requested paths (a file, or a
// folder and everything below it).
func selectEntries(all []archive.Entry, files []string) ([]archive.Entry, error) {
	if len(files) == 0 {
		return all, nil
	}
	var out []archive.Entry
	for _, f := range files {
		want := strings.Trim(strings.ReplaceAll(f, `\`, "/"), "/")
		n := 0
		for _, e := range all {
			p := strings.TrimSuffix(e.Path, "/")
			if p == want || strings.HasPrefix(p, want+"/") {
				out = append(out, e)
				n++
			}
		}
		if n == 0 {
			return nil, Errorf(KindUsage, nil, "err.notInBackup", f)
		}
	}
	return out, nil
}

// freeName returns rel if nothing exists there, otherwise
// "name (restored 2026-11-22).ext", "name (restored 2026-11-22 2).ext", ...
func freeName(root *os.Root, rel, day string) (string, error) {
	if _, err := root.Lstat(rel); errors.Is(err, fs.ErrNotExist) {
		return rel, nil
	} else if err != nil {
		return "", err
	}
	dir, base := path.Split(rel)
	ext := path.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	if stem == "" {
		stem, ext = base, ""
	}
	for i := 1; i < 1000; i++ {
		suffix := " (restored " + day + ")"
		if i > 1 {
			suffix = fmt.Sprintf(" (restored %s %d)", day, i)
		}
		cand := dir + stem + suffix + ext
		if _, err := root.Lstat(cand); errors.Is(err, fs.ErrNotExist) {
			return cand, nil
		}
	}
	return "", fmt.Errorf("no free name for %s", rel)
}

// restoreFile writes one file to a temporary name, checks its SHA-256 and
// gives it its final name.
func restoreFile(ctx context.Context, root *os.Root, src *openedSource, e archive.Entry, final string) error {
	rnd := make([]byte, 6)
	if _, err := rand.Read(rnd); err != nil {
		return Errorf(KindIO, err, "err.restoreWrite", final)
	}
	tmp := path.Join(path.Dir(final), ".usb-backup-restore-"+hex.EncodeToString(rnd)+".part")
	out, err := root.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return Errorf(KindIO, err, "err.restoreWrite", final)
	}
	ok := false
	defer func() {
		if !ok {
			_ = out.Close()
			_ = root.Remove(tmp)
		}
	}()
	rc, err := src.entryReader(e)
	if err != nil {
		return Errorf(KindVerify, err, "err.verify", e.Path)
	}
	h := sha256.New()
	lr := &io.LimitedReader{R: rc, N: int64(min(e.Size, 1<<62)) + 1}
	n, err := io.Copy(io.MultiWriter(out, h), ctxReader{ctx, lr})
	_ = rc.Close()
	if err != nil {
		if KindOf(err) == KindIO && ctx.Err() != nil {
			return Errorf(KindIO, err, "err.cancelled")
		}
		return Errorf(KindDecrypt, err, "err.decrypt", e.Path)
	}
	if uint64(n) != e.Size || e.SHA256 == "" || hex.EncodeToString(h.Sum(nil)) != e.SHA256 { //nolint:gosec // n is not negative
		return Errorf(KindVerify, errors.New("SHA-256 does not match the index"), "err.restoreHash", e.Path)
	}
	if err := out.Sync(); err != nil {
		return Errorf(KindIO, err, "err.restoreWrite", final)
	}
	if err := out.Close(); err != nil {
		return Errorf(KindIO, err, "err.restoreWrite", final)
	}
	if err := placeNoReplace(root, tmp, final); err != nil {
		return Errorf(KindIO, err, "err.restoreWrite", final)
	}
	ok = true
	if runtime.GOOS != "windows" && e.Mode != 0 {
		// No setuid/setgid/sticky, and never writable for group or others.
		_ = root.Chmod(final, fs.FileMode(e.Mode)&0o777&^0o022)
	}
	restoreOwner(root, final, e)
	if !e.Modified.IsZero() {
		_ = root.Chtimes(final, time.Now(), e.Modified)
	}
	return nil
}

// restoreOwner sets owner and group from the backup when running as root
// on Unix; otherwise the files belong to whoever restores them.
func restoreOwner(root *os.Root, rel string, e archive.Entry) {
	if runtime.GOOS == "windows" || os.Geteuid() != 0 || e.UID == nil || e.GID == nil {
		return
	}
	_ = root.Lchown(rel, int(*e.UID), int(*e.GID))
}

// placeNoReplace gives tmp the name final without ever replacing an
// existing file: a hard link fails if final exists (no race), then tmp is
// removed. Where hard links are not supported (FAT), it falls back to a
// rename after a check.
func placeNoReplace(root *os.Root, tmp, final string) error {
	err := root.Link(tmp, final)
	if err == nil {
		return root.Remove(tmp)
	}
	if errors.Is(err, fs.ErrExist) {
		return err
	}
	if _, err := root.Lstat(final); err == nil {
		return fs.ErrExist
	}
	return root.Rename(tmp, final)
}

// restoreLink recreates a symlink, but only a relative one that stays inside
// the restored tree. It returns a reason when it is skipped.
func restoreLink(root *os.Root, rel string, e archive.Entry, src *openedSource) string {
	if runtime.GOOS == "windows" {
		return "symlinks are not restored on Windows"
	}
	target := e.Link
	if target == "?" || target == "" {
		rc, err := src.entryReader(e)
		if err != nil {
			return "link cannot be read"
		}
		b, err := io.ReadAll(io.LimitReader(rc, 4096))
		_ = rc.Close()
		if err != nil {
			return "link cannot be read"
		}
		target = string(b)
	}
	// Only links that point down: no absolute target and no ".." at all.
	// A lexical check of ".." is not enough, because earlier links in the
	// same tree can make "a/l/.." mean something else.
	if filepath.IsAbs(target) || strings.HasPrefix(target, "/") || strings.ContainsAny(target, "\x00\\") {
		return "link points outside the restored folder"
	}
	for _, comp := range strings.Split(target, "/") {
		if comp == ".." {
			return "link points outside the restored folder"
		}
	}
	if _, err := root.Lstat(rel); err == nil {
		return "a file with this name already exists"
	}
	if err := root.Symlink(target, rel); err != nil {
		return "link cannot be created"
	}
	restoreOwner(root, rel, e)
	return ""
}
