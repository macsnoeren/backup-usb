package app

import (
	"context"
	"errors"

	"github.com/macsnoeren/backup-usb/internal/config"
	"github.com/macsnoeren/backup-usb/internal/disk"
	"github.com/macsnoeren/backup-usb/internal/secmem"
	"github.com/macsnoeren/backup-usb/internal/volume"
)

// Connected volume states for the disks command.
const (
	DiskRegistered = "registered"
	DiskUnknown    = "unknown"
	DiskPlain      = "plain"
)

// RegisteredDisk is a registration and whether it is connected.
type RegisteredDisk struct {
	disk.Registration
	Connected bool   `json:"connected"`
	Mount     string `json:"mount,omitempty"`
}

// ConnectedVolume is a connected volume and what it is.
type ConnectedVolume struct {
	volume.Volume
	State string `json:"state"`
	Name  string `json:"name,omitempty"`
}

// DisksReport lists registered and connected disks side by side.
type DisksReport struct {
	Registered []RegisteredDisk  `json:"registered"`
	Connected  []ConnectedVolume `json:"connected"`
}

// Disks lists the registered and the connected disks.
func Disks(ctx context.Context, deps *Deps, cfg *config.Config) (*DisksReport, error) {
	reg, err := LoadRegistry(deps.ConfigDir, cfg)
	if err != nil {
		return nil, err
	}
	pub, err := SigningKey(deps.ConfigDir)
	if err != nil {
		return nil, err
	}
	vols, err := deps.Finder.Volumes(ctx)
	if err != nil {
		return nil, Errorf(KindIO, err, "err.listDisks")
	}
	rep := &DisksReport{Registered: []RegisteredDisk{}, Connected: []ConnectedVolume{}}
	byName := map[string]string{}
	for _, v := range vols {
		cv := ConnectedVolume{Volume: v, State: DiskPlain}
		if v.Mount == "" && registeredSerial(reg, v.VolumeSerial) {
			if m, undo, err := deps.Finder.Mount(ctx, v); err == nil {
				deps.trackMount(undo)
				v = m
				cv.Volume = m
			}
		}
		if v.Mount != "" {
			id, raw, err := disk.ReadIDFile(v.Mount)
			switch {
			case errors.Is(err, disk.ErrNoIDFile):
			case err != nil:
				cv.State = DiskUnknown
			default:
				if r, err := disk.Match(v, id, raw, reg, cfg.RequireHardwareSerial, pub); err == nil {
					cv.State, cv.Name = DiskRegistered, r.Name
					byName[r.Name] = v.Mount
				} else {
					cv.State = DiskUnknown
				}
			}
		}
		rep.Connected = append(rep.Connected, cv)
	}
	for _, r := range reg.Disks {
		m, ok := byName[r.Name]
		rep.Registered = append(rep.Registered, RegisteredDisk{Registration: r, Connected: ok, Mount: m})
	}
	return rep, nil
}

// Forget removes a registration and signs the list again.
func Forget(deps *Deps, cfg *config.Config, name string, key *secmem.Buffer) error {
	if key == nil {
		return Errorf(KindUsage, nil, "err.needKeysInit")
	}
	reg, err := LoadRegistry(deps.ConfigDir, cfg)
	if err != nil {
		return err
	}
	r, i := reg.Find(name)
	if r == nil {
		return Errorf(KindUsage, nil, "err.notRegistered", name)
	}
	reg.Disks = append(reg.Disks[:i], reg.Disks[i+1:]...)
	if err := SaveRegistry(deps.ConfigDir, cfg, reg, key); err != nil {
		return err
	}
	deps.Log.Info("disk forgotten", "name", name)
	return nil
}
