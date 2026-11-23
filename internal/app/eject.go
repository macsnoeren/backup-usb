package app

import (
	"context"

	"github.com/macsnoeren/backup-usb/internal/config"
)

// Eject safely removes the connected backup disk (or the one matching
// target).
func Eject(ctx context.Context, deps *Deps, cfg *config.Config, target string) (string, error) {
	disks, err := connectedDisks(ctx, deps, cfg, target)
	if err != nil {
		return "", err
	}
	if len(disks) != 1 {
		return "", Errorf(KindUsage, nil, "err.oneDisk")
	}
	c := disks[0]
	if err := deps.Finder.Eject(ctx, c.Vol); err != nil {
		return "", Errorf(KindIO, err, "err.eject", c.Reg.Name)
	}
	deps.Log.Info("disk ejected", "disk", c.Reg.Name)
	return c.Reg.Name, nil
}
