// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Maurice Snoeren
//
// This file is part of USB Backup.
//
// USB Backup is free software: you can redistribute it and/or modify it under
// the terms of the GNU General Public License as published by the Free Software
// Foundation, either version 3 of the License, or (at your option) any later
// version.
//
// USB Backup is distributed in the hope that it will be useful, but WITHOUT ANY
// WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A
// PARTICULAR PURPOSE. See the GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License along with
// USB Backup. If not, see <https://www.gnu.org/licenses/>.

package volume

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

var (
	kernel32                 = syscall.NewLazyDLL("kernel32.dll")
	procGetLogicalDrives     = kernel32.NewProc("GetLogicalDrives")
	procGetDriveTypeW        = kernel32.NewProc("GetDriveTypeW")
	procGetVolumeInformation = kernel32.NewProc("GetVolumeInformationW")
	procGetDiskFreeSpaceEx   = kernel32.NewProc("GetDiskFreeSpaceExW")
	procGetSystemDirectory   = kernel32.NewProc("GetSystemDirectoryW")
)

const (
	driveRemovable = 2
	driveFixed     = 3

	ioctlStorageQueryProperty    = 0x002D1400
	ioctlDiskGetPartitionInfoEx  = 0x00070048
	ioctlDiskGetDriveLayoutEx    = 0x00070050
	fsctlLockVolume              = 0x00090018
	fsctlDismountVolume          = 0x00090020
	ioctlStorageMediaRemoval     = 0x002D4804
	ioctlStorageEjectMedia       = 0x002D4808
	partitionStyleMBR            = 0
	partitionStyleGPT            = 1
	createNoWindow               = 0x08000000
	errorNotReady                = 21
	fileShareReadWrite           = syscall.FILE_SHARE_READ | syscall.FILE_SHARE_WRITE
	openExisting                 = syscall.OPEN_EXISTING
	genericReadWrite             = syscall.GENERIC_READ | syscall.GENERIC_WRITE
	maxVolumeInfoName            = syscall.MAX_PATH + 1
	storageDeviceProperty        = 0
	propertyStandardQuery        = 0
	storageDescriptorBufferBytes = 1024
)

type windowsFinder struct{}

// NewPlatformFinder returns the finder for this platform.
func NewPlatformFinder() Finder { return windowsFinder{} }

func systemDrive() string {
	if d := os.Getenv("SystemDrive"); d != "" {
		return strings.ToUpper(d)
	}
	return "C:"
}

// Volumes lists removable and fixed data volumes with a drive letter, not
// the system drive.
func (windowsFinder) Volumes(context.Context) ([]Volume, error) {
	mask, _, _ := procGetLogicalDrives.Call()
	var out []Volume
	for _, letter := range DriveLetters(uint32(mask)) { //nolint:gosec // GetLogicalDrives returns a 32-bit mask
		if letter == systemDrive() {
			continue
		}
		root := letter + `\`
		rootp, _ := syscall.UTF16PtrFromString(root)
		t, _, _ := procGetDriveTypeW.Call(uintptr(unsafe.Pointer(rootp))) //nolint:gosec // Win32 call
		if t != driveRemovable && t != driveFixed {
			continue
		}
		v, err := volumeInfo(letter)
		if err != nil {
			continue // no media, or not ready
		}
		v.Removable = t == driveRemovable || v.Removable
		out = append(out, v)
	}
	return out, nil
}

func volumeInfo(letter string) (Volume, error) {
	root, _ := syscall.UTF16PtrFromString(letter + `\`)
	label := make([]uint16, maxVolumeInfoName)
	fs := make([]uint16, maxVolumeInfoName)
	var serial, maxComp, flags uint32
	r, _, err := procGetVolumeInformation.Call(uintptr(unsafe.Pointer(root)), //nolint:gosec // Win32 call
		uintptr(unsafe.Pointer(&label[0])), uintptr(len(label)), //nolint:gosec // Win32 call
		uintptr(unsafe.Pointer(&serial)), uintptr(unsafe.Pointer(&maxComp)), uintptr(unsafe.Pointer(&flags)), //nolint:gosec // Win32 call
		uintptr(unsafe.Pointer(&fs[0])), uintptr(len(fs))) //nolint:gosec // Win32 call
	if r == 0 {
		return Volume{}, err
	}
	v := Volume{
		Mount:        letter + `\`,
		Device:       letter,
		Label:        UTF16ToString(label),
		FileSystem:   UTF16ToString(fs),
		VolumeSerial: FormatVolumeSerial(serial),
	}
	if h, err := openVolume(letter, 0); err == nil {
		v.PartitionID = partitionID(h)
		desc := storageDescriptor(h)
		v.HardwareSerial = DescriptorSerial(desc)
		rem, usb := DescriptorRemovable(desc)
		v.Removable = rem || usb
		_ = syscall.CloseHandle(h)
	}
	v.FreeBytes, v.TotalBytes, _ = diskSpace(letter + `\`)
	return v, nil
}

func openVolume(letter string, access uint32) (syscall.Handle, error) {
	p, _ := syscall.UTF16PtrFromString(`\\.\` + letter)
	return syscall.CreateFile(p, access, fileShareReadWrite, nil, openExisting, 0, 0)
}

func ioctl(h syscall.Handle, code uint32, in []byte, out []byte) (uint32, error) {
	var n uint32
	var inp, outp *byte
	if len(in) > 0 {
		inp = &in[0]
	}
	if len(out) > 0 {
		outp = &out[0]
	}
	err := syscall.DeviceIoControl(h, code, inp, uint32(len(in)), outp, uint32(len(out)), &n, nil) //nolint:gosec // lengths are small
	return n, err
}

// storageDescriptor returns the STORAGE_DEVICE_DESCRIPTOR of a volume.
func storageDescriptor(h syscall.Handle) []byte {
	query := make([]byte, 12) // PropertyId, QueryType, AdditionalParameters
	binary.LittleEndian.PutUint32(query[0:], storageDeviceProperty)
	binary.LittleEndian.PutUint32(query[4:], propertyStandardQuery)
	out := make([]byte, storageDescriptorBufferBytes)
	n, err := ioctl(h, ioctlStorageQueryProperty, query, out)
	if err != nil {
		return nil
	}
	return out[:n]
}

// partitionID returns the GPT partition GUID, or for MBR the disk
// signature and partition number.
func partitionID(h syscall.Handle) string {
	out := make([]byte, 256)
	n, err := ioctl(h, ioctlDiskGetPartitionInfoEx, nil, out)
	if err != nil || n < 48 {
		return ""
	}
	// PARTITION_INFORMATION_EX: PartitionStyle(4) pad(4) StartingOffset(8)
	// PartitionLength(8) PartitionNumber(4) RewritePartition(1)
	// IsServicePartition(1) pad(2), then the union at offset 32.
	style := binary.LittleEndian.Uint32(out[0:4])
	number := binary.LittleEndian.Uint32(out[24:28])
	switch style {
	case partitionStyleGPT:
		// PARTITION_INFORMATION_GPT: PartitionType GUID(16), PartitionId GUID(16)
		if n < 32+32 {
			return ""
		}
		return FormatGUID(out[48:64])
	case partitionStyleMBR:
		layout := make([]byte, 4096)
		if m, err := ioctl(h, ioctlDiskGetDriveLayoutEx, nil, layout); err == nil && m >= 12 {
			// DRIVE_LAYOUT_INFORMATION_EX: PartitionStyle(4) PartitionCount(4)
			// DRIVE_LAYOUT_INFORMATION_MBR.Signature(4)
			return FormatMBRPartition(binary.LittleEndian.Uint32(layout[8:12]), number)
		}
		start := binary.LittleEndian.Uint64(out[8:16])
		return fmt.Sprintf("MBR:offset%d:%d", start, number)
	}
	return ""
}

func diskSpace(root string) (free, total uint64, err error) {
	p, _ := syscall.UTF16PtrFromString(root)
	var avail, tot, totalFree uint64
	r, _, e := procGetDiskFreeSpaceEx.Call(uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(&avail)), uintptr(unsafe.Pointer(&tot)), uintptr(unsafe.Pointer(&totalFree))) //nolint:gosec // Win32 call
	if r == 0 {
		return 0, 0, e
	}
	return avail, tot, nil
}

// Space implements Finder.
func (windowsFinder) Space(_ context.Context, v Volume) (uint64, uint64, error) {
	return diskSpace(v.Mount)
}

func systemDir() string {
	buf := make([]uint16, syscall.MAX_PATH)
	n, _, _ := procGetSystemDirectory.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf))) //nolint:gosec // Win32 call
	if n == 0 || int(n) > len(buf) {
		return `C:\Windows\System32`
	}
	return UTF16ToString(buf)
}

// Health asks Windows for the HealthStatus of the disk (as Get-Disk shows
// it) through a hidden PowerShell, by full path and without a shell.
func (windowsFinder) Health(ctx context.Context, v Volume) Health {
	letter := strings.TrimSuffix(v.Device, ":")
	if len(letter) != 1 || letter[0] < 'A' || letter[0] > 'Z' {
		return Unknown
	}
	ps := filepath.Join(systemDir(), "WindowsPowerShell", "v1.0", "powershell.exe")
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	// The drive letter is validated above; it is the only input.
	args := []string{"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command",
		"(Get-Partition -DriveLetter " + letter + " | Get-Disk).HealthStatus"}
	cmd := exec.CommandContext(ctx, ps, args...) //nolint:gosec // full path; the only input is a validated drive letter
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	out, err := cmd.Output()
	if err != nil {
		return Unknown
	}
	return ParseHealthOutput(string(out))
}

// Mount implements Finder: Windows mounts volumes itself.
func (windowsFinder) Mount(_ context.Context, v Volume) (Volume, func() error, error) {
	if v.Mount == "" {
		return v, func() error { return nil }, ErrNotMounted
	}
	return v, func() error { return nil }, nil
}
