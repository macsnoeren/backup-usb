package cli

import "github.com/macsnoeren/backup-usb/internal/volume"

func platformFinder() volume.Finder { return volume.NewPlatformFinder() }
