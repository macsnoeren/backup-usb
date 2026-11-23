package app

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"time"

	"github.com/macsnoeren/backup-usb/internal/config"
	"github.com/macsnoeren/backup-usb/internal/disk"
	"github.com/macsnoeren/backup-usb/internal/fsx"
	"github.com/macsnoeren/backup-usb/internal/secmem"
	"github.com/macsnoeren/backup-usb/internal/sign"
	"github.com/macsnoeren/backup-usb/internal/volume"
)

// EnrollOptions are the arguments of enroll.
type EnrollOptions struct {
	Target string
	Name   string
	Weeks  string
	Force  bool
	Key    *secmem.Buffer // operator signing key
}

// NewUUID returns a random version 4 UUID.
func NewUUID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}

// findVolume returns the connected volume matching target, mounted.
func findVolume(ctx context.Context, deps *Deps, target string) (volume.Volume, error) {
	vols, err := deps.Finder.Volumes(ctx)
	if err != nil {
		return volume.Volume{}, Errorf(KindIO, err, "err.listDisks")
	}
	for _, v := range vols {
		if !matchesTarget(v, target) {
			continue
		}
		if v.Mount == "" {
			m, _, err := deps.Finder.Mount(ctx, v)
			if err != nil {
				return v, Errorf(KindDiskCheck, err, "err.mount", target)
			}
			v = m
		}
		return v, nil
	}
	return volume.Volume{}, Errorf(KindNoDisk, nil, "err.noSuchDisk", target)
}

// Enroll registers a disk: it writes a signed Backup/disk-id.json with a new
// UUID and the serial numbers of the disk, and adds the disk to the signed
// registration list.
func Enroll(ctx context.Context, deps *Deps, cfg *config.Config, o EnrollOptions) (*disk.Registration, error) {
	if o.Key == nil {
		return nil, Errorf(KindUsage, nil, "err.needKeysInit")
	}
	if !disk.ValidName(o.Name) {
		return nil, Errorf(KindUsage, nil, "err.diskName", o.Name)
	}
	switch {
	case o.Weeks == "" && cfg.Rotation == "oddEven":
		return nil, Errorf(KindUsage, nil, "err.weeksNeeded")
	case o.Weeks != "" && o.Weeks != "odd" && o.Weeks != "even":
		return nil, Errorf(KindUsage, nil, "err.weeksNeeded")
	}
	reg, err := LoadRegistry(deps.ConfigDir, cfg)
	if err != nil {
		return nil, err
	}
	if r, i := reg.Find(o.Name); r != nil {
		if !o.Force {
			return nil, Errorf(KindUsage, nil, "err.diskRegistered", o.Name)
		}
		reg.Disks = append(reg.Disks[:i], reg.Disks[i+1:]...)
	}
	v, err := findVolume(ctx, deps, o.Target)
	if err != nil {
		return nil, err
	}
	if v.VolumeSerial == "" || v.PartitionID == "" {
		return nil, Errorf(KindDiskCheck, nil, "err.noSerials", o.Target)
	}
	for i := range reg.Disks {
		if strings.EqualFold(reg.Disks[i].VolumeSerial, v.VolumeSerial) && strings.EqualFold(reg.Disks[i].PartitionID, v.PartitionID) {
			if !o.Force {
				return nil, Errorf(KindUsage, nil, "err.diskRegistered", reg.Disks[i].Name)
			}
			reg.Disks = append(reg.Disks[:i], reg.Disks[i+1:]...)
			break
		}
	}
	id, err := NewUUID()
	if err != nil {
		return nil, Errorf(KindIO, err, "err.keygen")
	}
	r := disk.Registration{
		Name: o.Name, Weeks: o.Weeks, ID: id, VolumeSerial: v.VolumeSerial, PartitionID: v.PartitionID,
		HardwareSerial: v.HardwareSerial, Enrolled: deps.Now().Format(time.DateOnly),
	}
	// Registry first: if writing the disk fails, the new entry only makes
	// a disk unknown that was not enrolled yet; the other order could leave
	// a disk whose new UUID is in no registry.
	reg.Disks = append(reg.Disks, r)
	if err := SaveRegistry(deps.ConfigDir, cfg, reg, o.Key); err != nil {
		return nil, err
	}
	if err := writeIDFile(v.Mount, r, o.Key); err != nil {
		return nil, err
	}
	deps.Log.Info("disk enrolled", "name", r.Name, "weeks", r.Weeks, "id", r.ID, "volumeSerial", r.VolumeSerial)
	return &r, nil
}

func writeIDFile(mount string, r disk.Registration, key *secmem.Buffer) error {
	idf := disk.IDFile{Version: 1, ID: r.ID, Name: r.Name, VolumeSerial: r.VolumeSerial, PartitionID: r.PartitionID, HardwareSerial: r.HardwareSerial, Enrolled: r.Enrolled}
	raw, err := json.Marshal(idf)
	if err != nil {
		return Errorf(KindIO, err, "err.write")
	}
	signed, err := sign.SignJSON(key.Bytes(), sign.PurposeDiskID, raw)
	if err != nil {
		return Errorf(KindIO, err, "err.write")
	}
	root, err := fsx.OpenDiskRoot(mount, disk.IDDir, true)
	if err != nil {
		return Errorf(KindDiskCheck, err, "err.notWritable")
	}
	defer func() { _ = root.Close() }()
	if _, err := root.Lstat(disk.IDFileName); err == nil {
		if err := fsx.Unprotect(root, disk.IDFileName); err != nil {
			return Errorf(KindIO, err, "err.write")
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return Errorf(KindIO, err, "err.write")
	}
	if err := fsx.RootWriteFileAtomic(root, disk.IDFileName, signed, 0o644); err != nil {
		return Errorf(KindIO, err, "err.write")
	}
	// Read-only/hidden is a convenience; some file systems refuse it.
	_ = fsx.Protect(root, disk.IDFileName)
	return nil
}
