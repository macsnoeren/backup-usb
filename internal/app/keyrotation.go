package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"sort"
	"strings"

	"filippo.io/age"

	"github.com/macsnoeren/backup-usb/internal/archive"
	"github.com/macsnoeren/backup-usb/internal/config"
	"github.com/macsnoeren/backup-usb/internal/crypt"
	"github.com/macsnoeren/backup-usb/internal/fsx"
	"github.com/macsnoeren/backup-usb/internal/secmem"
	"github.com/macsnoeren/backup-usb/internal/sign"
	"github.com/macsnoeren/backup-usb/internal/vault"
)

// RotateResult describes a key rotation.
type RotateResult struct {
	NewKey       string   `json:"newKey"`
	Fingerprint  string   `json:"fingerprint"`
	Retired      string   `json:"retired"`
	PendingDisks []string `json:"pendingDisks"`
	Full         bool     `json:"full"`
}

// RotateKeys (caller holds the lock and opened the vault after it) creates
// a new operational key: the old one is retired with all
// registered disks pending, the recipients file is replaced (new
// operational + break-glass) and signed. New backups use the new key at
// once; existing ones are re-wrapped per disk with RewrapDisk.
func RotateKeys(deps *Deps, cfg *config.Config, v *vault.Vault, vaultPath string, pw *secmem.Buffer, full bool) (*RotateResult, error) {
	old := v.Active()
	if old == nil {
		return nil, Errorf(KindDecrypt, nil, "err.vaultDamaged")
	}
	reg, err := LoadRegistry(deps.ConfigDir, cfg)
	if err != nil {
		return nil, err
	}
	pending := []string{}
	for _, d := range reg.Disks {
		pending = append(pending, d.Name)
	}
	idBuf, rcp, err := generateIdentity()
	if err != nil {
		return nil, Errorf(KindIO, err, "err.keygen")
	}
	defer idBuf.Wipe()
	oldID := old.ID
	newID := nextKeyID(v)
	k, err := v.AddKey(newID, idBuf.Bytes(), deps.Now())
	if err != nil {
		return nil, Errorf(KindIO, err, "err.keygen")
	}
	ok := v.Key(oldID)
	ok.PendingDisks = pending
	ok.FullRewrap = full
	if err := v.Save(vaultPath, pw); err != nil {
		return nil, Errorf(KindIO, err, "err.write")
	}
	sk := v.SigningKey()
	defer sk.Wipe()
	if err := writeSigned(cfg.RecipientsPath(deps.ConfigDir), RecipientsFile(k.ID, rcp, v.Meta.BreakglassRecipient), sk, sign.PurposeRecipients); err != nil {
		return nil, err
	}
	deps.Log.Info("key rotated", "new", newID, "retired", oldID, "pending", strings.Join(pending, ","), "full", full)
	return &RotateResult{NewKey: newID, Fingerprint: k.Fingerprint, Retired: oldID, PendingDisks: pending, Full: full}, nil
}

func nextKeyID(v *vault.Vault) string {
	n := 1
	for _, k := range v.Meta.Keys {
		var i int
		if _, err := fmt.Sscanf(k.ID, "k%d", &i); err == nil && i >= n {
			n = i + 1
		}
	}
	return fmt.Sprintf("k%d", n)
}

// RewrapReport describes the re-wrap of one disk.
type RewrapReport struct {
	Disk      string   `json:"disk"`
	Rewrapped int      `json:"rewrapped"`
	Current   int      `json:"alreadyCurrent"`
	Full      bool     `json:"full"`
	Folders   []string `json:"folders"`
	// OldKeysFree lists retired keys no disk needs any more.
	OldKeysFree []string `json:"oldKeysFree"`
}

// RewrapDisk re-wraps (or with full re-encrypts) every encrypted archive
// and index on the connected disk for the current recipients, checks each
// file with the new key, updates the .sha256 files and keys.json, and takes
// the disk off the pending lists in the vault (saved with pw). The caller
// holds the single-instance lock and opened the vault after taking it, so
// no other key change can be overwritten by this one.
func RewrapDisk(ctx context.Context, deps *Deps, cfg *config.Config, target string, v *vault.Vault, vaultPath string, pw *secmem.Buffer, forceFull bool) (*RewrapReport, error) {
	enc, err := encryptionFor(deps.ConfigDir, cfg)
	if err != nil {
		return nil, err
	}
	if enc == nil {
		return nil, Errorf(KindUsage, nil, "err.noRecipients", cfg.RecipientsPath(deps.ConfigDir))
	}
	active := v.Active()
	check, err := v.Identity(active.ID)
	if err != nil {
		return nil, Errorf(KindDecrypt, err, "err.vaultDamaged")
	}
	if !strings.EqualFold(enc.Operational, active.Recipient) {
		return nil, Errorf(KindDecrypt, errors.New("recipients file and vault disagree on the active key"), "err.otherVault")
	}
	oldIDs, err := v.Identities()
	if err != nil {
		return nil, Errorf(KindDecrypt, err, "err.vaultDamaged")
	}
	disks, err := connectedDisks(ctx, deps, cfg, target)
	if err != nil {
		return nil, err
	}
	if len(disks) != 1 {
		return nil, Errorf(KindUsage, nil, "err.oneDisk")
	}
	c := disks[0]
	full := forceFull
	for _, k := range v.Meta.Keys {
		if k.FullRewrap && containsFold(k.PendingDisks, c.Reg.Name) {
			full = true
		}
	}
	rep := &RewrapReport{Disk: c.Reg.Name, Full: full, Folders: []string{}, OldKeysFree: []string{}}
	root, err := fsx.OpenDiskRoot(c.Vol.Mount, cfg.TargetFolder, false)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, Errorf(KindIO, err, "err.read")
	}
	if err == nil {
		defer func() { _ = root.Close() }()
		if err := rewrapFolders(ctx, deps, cfg, root, rep, oldIDs, enc, check, full); err != nil {
			return rep, err
		}
	}
	for i := range v.Meta.Keys {
		k := &v.Meta.Keys[i]
		k.PendingDisks = removeFold(k.PendingDisks, c.Reg.Name)
		if k.Status == vault.StatusRetired && len(k.PendingDisks) == 0 {
			rep.OldKeysFree = append(rep.OldKeysFree, k.ID)
		}
	}
	if err := v.Save(vaultPath, pw); err != nil {
		return rep, Errorf(KindIO, err, "err.write")
	}
	deps.Log.Info("disk re-wrapped", "disk", c.Reg.Name, "files", rep.Rewrapped, "full", full)
	return rep, nil
}

func rewrapFolders(ctx context.Context, deps *Deps, cfg *config.Config, root *os.Root, rep *RewrapReport, oldIDs []age.Identity, enc *encryption, check age.Identity, full bool) error {
	state, err := ReadDiskState(root)
	if err != nil {
		state = &DiskState{Version: 1, Folders: map[string]FolderRecord{}}
	}
	folders, err := periodFolders(root, cfg)
	if err != nil {
		return Errorf(KindIO, err, "err.read")
	}
	for _, f := range folders {
		names, err := ageFilesIn(root, f.Rel)
		if err != nil {
			return Errorf(KindIO, err, "err.read")
		}
		changed := false
		for _, n := range names {
			if err := ctx.Err(); err != nil {
				return Errorf(KindIO, err, "err.cancelled")
			}
			rel := path.Join(f.Rel, n)
			if !full && crypt.OpensWith(root, rel, check) {
				rep.Current++
				continue
			}
			deps.emit(Event{Event: "state", State: "rewrapping", Tag: n, Disk: rep.Disk})
			isArchive := strings.HasSuffix(n, ".zip"+crypt.Suffix)
			switch {
			case full:
				// Decrypting checks every chunk, so damage is found.
				err = crypt.Reencrypt(root, rel, oldIDs, enc.age, check)
			case isArchive:
				var want string
				want, err = shaOf(root, rel)
				if err == nil {
					err = crypt.Rewrap(root, rel, oldIDs, enc.age, check, want)
				}
			default:
				// An index has no .sha256: decrypt it completely first.
				err = checkDecrypts(root, rel, oldIDs)
				if err == nil {
					err = crypt.Rewrap(root, rel, oldIDs, enc.age, check, "")
				}
			}
			if err != nil {
				if errors.Is(err, crypt.ErrDamaged) {
					return Errorf(KindVerify, err, "err.rewrapDamaged", rel)
				}
				if errors.Is(err, crypt.ErrDecrypt) {
					return Errorf(KindDecrypt, err, "err.rewrap", rel)
				}
				return Errorf(KindIO, err, "err.rewrap", rel)
			}
			if isArchive {
				if err := rewriteSHA(ctx, root, rel); err != nil {
					return err
				}
			}
			rep.Rewrapped++
			changed = true
		}
		if changed || len(names) > 0 {
			rec := state.Folders[f.Rel]
			rec.Keys = enc.Fingerprints()
			state.Folders[f.Rel] = rec
			if changed {
				rep.Folders = append(rep.Folders, f.Rel)
			}
		}
	}
	if err := state.Write(root); err != nil {
		return Errorf(KindIO, err, "err.write")
	}
	return nil
}

// ageFilesIn lists the encrypted archives and indexes in a folder.
func ageFilesIn(root *os.Root, folder string) ([]string, error) {
	d, err := root.Open(folder)
	if err != nil {
		return nil, err
	}
	entries, err := d.ReadDir(-1)
	_ = d.Close()
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		n := e.Name()
		if e.Type().IsRegular() && strings.HasSuffix(n, crypt.Suffix) && (archiveFileRe.MatchString(n) || strings.HasSuffix(n, ".index"+crypt.Suffix)) {
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out, nil
}

// shaOf reads the SHA-256 from the .sha256 file of rel.
func shaOf(root *os.Root, rel string) (string, error) {
	data, err := readLimited(root, rel+".sha256", 4096)
	if err != nil {
		return "", fmt.Errorf("%w: .sha256 missing", crypt.ErrDamaged)
	}
	sum, _, _ := strings.Cut(strings.TrimPrefix(string(data), `\`), "  ")
	if len(sum) != 64 {
		return "", fmt.Errorf("%w: .sha256 invalid", crypt.ErrDamaged)
	}
	return sum, nil
}

// checkDecrypts decrypts rel completely (age checks every chunk).
func checkDecrypts(root *os.Root, rel string, ids []age.Identity) error {
	f, err := fsx.OpenRegular(root, rel)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }() // read-only
	r, err := age.Decrypt(f, ids...)
	if err != nil {
		return fmt.Errorf("%w: %w", crypt.ErrDecrypt, err)
	}
	if _, err := io.Copy(io.Discard, r); err != nil {
		return fmt.Errorf("%w: %w", crypt.ErrDamaged, err)
	}
	return nil
}

func rewriteSHA(ctx context.Context, root *os.Root, rel string) error {
	sum, _, err := HashFile(ctx, root, rel)
	if err != nil {
		return Errorf(KindIO, err, "err.read")
	}
	if err := fsx.RootWriteFileAtomic(root, rel+".sha256", []byte(archive.ManifestLine(sum, path.Base(rel))), 0o644); err != nil {
		return Errorf(KindIO, err, "err.write")
	}
	return nil
}

func removeFold(list []string, s string) []string {
	out := list[:0:0]
	for _, x := range list {
		if !strings.EqualFold(x, s) {
			out = append(out, x)
		}
	}
	if out == nil {
		out = []string{}
	}
	return out
}
