package volume

import (
	"context"
	"errors"
)

// Health is the health status of a disk as reported by the operating system.
type Health string

// Health values. Unknown means the status could not be read.
const (
	Healthy   Health = "Healthy"
	Warning   Health = "Warning"
	Unhealthy Health = "Unhealthy"
	Unknown   Health = "Unknown"
)

// ParseHealth maps a status string to a Health value; anything unrecognised
// is Unknown.
func ParseHealth(s string) Health {
	switch Health(s) {
	case Healthy, Warning, Unhealthy:
		return Health(s)
	}
	return Unknown
}

// Volume describes one candidate backup volume.
type Volume struct {
	// Mount is where the file system is reachable: "E:\" on Windows, a
	// directory on Linux. Empty if the volume is not mounted.
	Mount string `json:"mount"`
	// Device is the device name: "E:" on Windows, "/dev/sdb1" on Linux.
	Device string `json:"device"`
	// Label is the volume label, for display only.
	Label string `json:"label"`
	// FileSystem is the file system name, for example "NTFS", "exFAT",
	// "FAT32", "vfat" or "ext4".
	FileSystem string `json:"fileSystem"`
	// VolumeSerial is the Windows volume serial number or the Linux file
	// system UUID.
	VolumeSerial string `json:"volumeSerial"`
	// PartitionID is the GPT partition GUID, the MBR disk signature or the
	// Linux PARTUUID.
	PartitionID string `json:"partitionId"`
	// HardwareSerial is the USB device serial number, empty if unknown.
	HardwareSerial string `json:"hardwareSerial"`
	// Removable reports whether the OS sees the volume as removable.
	Removable bool `json:"removable"`
	// FreeBytes and TotalBytes are the space on the volume.
	FreeBytes  uint64 `json:"freeBytes"`
	TotalBytes uint64 `json:"totalBytes"`
}

// ErrNotMounted is returned when a volume has no mount point and cannot be
// mounted.
var ErrNotMounted = errors.New("volume is not mounted")

// Finder finds volumes and reports their properties. Production code uses
// the platform implementation; tests use the fake.
type Finder interface {
	// Volumes lists the candidate volumes (removable or fixed data volumes,
	// not the system volume).
	Volumes(ctx context.Context) ([]Volume, error)
	// Space returns the free and total bytes of a mounted volume.
	Space(ctx context.Context, v Volume) (free, total uint64, err error)
	// Health returns the disk health, Unknown if it cannot be determined.
	Health(ctx context.Context, v Volume) Health
	// Mount makes sure the volume is mounted. The returned function undoes
	// a mount that Mount itself performed; it is never nil.
	Mount(ctx context.Context, v Volume) (Volume, func() error, error)
	// Eject safely removes the volume.
	Eject(ctx context.Context, v Volume) error
}

// IsFAT32 reports whether a file system name denotes FAT32 (4 GB file
// limit).
func IsFAT32(fs string) bool {
	switch fs {
	case "FAT32", "FAT", "vfat", "fat32", "msdos":
		return true
	}
	return false
}
