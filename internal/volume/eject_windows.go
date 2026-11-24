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
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

// The libraries are loaded by full path, so nothing next to the program is
// loaded instead.
var (
	setupapi                  = syscall.NewLazyDLL(filepath.Join(systemDir(), "setupapi.dll"))
	procGetClassDevs          = setupapi.NewProc("SetupDiGetClassDevsW")
	procEnumDeviceInterfaces  = setupapi.NewProc("SetupDiEnumDeviceInterfaces")
	procGetInterfaceDetail    = setupapi.NewProc("SetupDiGetDeviceInterfaceDetailW")
	procDestroyDeviceInfoList = setupapi.NewProc("SetupDiDestroyDeviceInfoList")
	cfgmgr32                  = syscall.NewLazyDLL(filepath.Join(systemDir(), "cfgmgr32.dll"))
	procCMGetParent           = cfgmgr32.NewProc("CM_Get_Parent")
	procCMGetDeviceID         = cfgmgr32.NewProc("CM_Get_Device_IDW")
	procCMRequestEject        = cfgmgr32.NewProc("CM_Request_Device_EjectW")
)

const (
	ioctlStorageGetDeviceNumber = 0x002D1080
	digcfPresent                = 0x02
	digcfDeviceInterface        = 0x10
	invalidHandle               = ^uintptr(0)
	ejectAttempts               = 20
	ejectPause                  = 500 * time.Millisecond
)

// guidDisk is GUID_DEVINTERFACE_DISK.
var guidDisk = syscall.GUID{Data1: 0x53f56307, Data2: 0xb6bf, Data3: 0x11d0, Data4: [8]byte{0x94, 0xf2, 0x00, 0xa0, 0xc9, 0x1e, 0xfb, 0x8b}}

// deviceInterfaceData is SP_DEVICE_INTERFACE_DATA.
type deviceInterfaceData struct {
	size     uint32
	guid     syscall.GUID
	flags    uint32
	reserved uintptr
}

// deviceInfoData is SP_DEVINFO_DATA.
type deviceInfoData struct {
	size     uint32
	guid     syscall.GUID
	devInst  uint32
	reserved uintptr
}

// Eject makes the disk safe to unplug. It first asks Windows to remove the
// USB device, like "Safely remove hardware": that works for hard disks too
// and needs no administrator rights. If that is not possible (card readers)
// it dismounts the volume and ejects the media. Another program may still
// look at the new files for a moment (virus scanner, Explorer), so it tries
// again for some seconds.
func (windowsFinder) Eject(ctx context.Context, v Volume) error {
	if !v.Removable {
		return errors.New("not a removable disk")
	}
	letter := strings.TrimSuffix(v.Device, `\`)
	var err error
	for i := 0; i < ejectAttempts; i++ {
		if err = ejectDevice(letter); err == nil {
			return nil
		}
		if errors.Is(err, errNotUSB) {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(ejectPause):
		}
	}
	if merr := ejectMedia(letter); merr != nil {
		return errors.Join(err, merr)
	}
	return nil
}

var errNotUSB = errors.New("not a USB device")

// deviceNumber is the type and number of the disk behind a volume or disk.
func deviceNumber(h syscall.Handle) (uint64, error) {
	out := make([]byte, 12) // STORAGE_DEVICE_NUMBER
	n, err := ioctl(h, ioctlStorageGetDeviceNumber, nil, out)
	if err != nil {
		return 0, fmt.Errorf("device number: %w", err)
	}
	if n < 8 {
		return 0, errors.New("device number: short answer")
	}
	return binary.LittleEndian.Uint64(out[:8]), nil
}

// ejectDevice removes the USB device that holds the volume.
func ejectDevice(letter string) error {
	h, err := openVolume(letter, 0)
	if err != nil {
		return fmt.Errorf("open volume: %w", err)
	}
	want, err := deviceNumber(h)
	_ = syscall.CloseHandle(h) // open handles would veto the removal
	if err != nil {
		return err
	}
	inst, err := diskInstance(want)
	if err != nil {
		return err
	}
	var parent uint32
	if r, _, _ := procCMGetParent.Call(uintptr(unsafe.Pointer(&parent)), uintptr(inst), 0); r != 0 { //nolint:gosec // Win32 call
		return fmt.Errorf("parent device: error %d", r)
	}
	id := make([]uint16, 260)
	if r, _, _ := procCMGetDeviceID.Call(uintptr(parent), uintptr(unsafe.Pointer(&id[0])), uintptr(len(id)), 0); r != 0 { //nolint:gosec // Win32 call
		return fmt.Errorf("parent device: error %d", r)
	}
	// Never ask for the removal of anything but a USB device, such as the
	// controller an internal disk is attached to.
	if !IsUSBDevice(UTF16ToString(id)) {
		return errNotUSB
	}
	// With a buffer for the reason, Windows shows no window of its own.
	var veto uint32
	name := make([]uint16, 260)
	r, _, _ := procCMRequestEject.Call(uintptr(parent), uintptr(unsafe.Pointer(&veto)), uintptr(unsafe.Pointer(&name[0])), uintptr(len(name)), 0) //nolint:gosec // Win32 call
	if r != 0 || veto != 0 {
		return fmt.Errorf("device in use (error %d, reason %d)", r, veto)
	}
	return nil
}

// diskInstance finds the device instance of the disk with this number.
func diskInstance(want uint64) (uint32, error) {
	set, _, err := procGetClassDevs.Call(uintptr(unsafe.Pointer(&guidDisk)), 0, 0, digcfPresent|digcfDeviceInterface) //nolint:gosec // Win32 call
	if set == invalidHandle {
		return 0, fmt.Errorf("list disks: %w", err)
	}
	defer procDestroyDeviceInfoList.Call(set) //nolint:errcheck // nothing to do about it
	for i := uintptr(0); i < 256; i++ {
		ifc := deviceInterfaceData{}
		ifc.size = uint32(unsafe.Sizeof(ifc))
		if r, _, _ := procEnumDeviceInterfaces.Call(set, 0, uintptr(unsafe.Pointer(&guidDisk)), i, uintptr(unsafe.Pointer(&ifc))); r == 0 { //nolint:gosec // Win32 call
			break
		}
		// SP_DEVICE_INTERFACE_DETAIL_DATA_W: cbSize (8 on 64-bit Windows),
		// then the path at offset 4.
		detail := make([]uint16, 1024)
		*(*uint32)(unsafe.Pointer(&detail[0])) = 8 //nolint:gosec // Win32 structure
		info := deviceInfoData{}
		info.size = uint32(unsafe.Sizeof(info))
		r, _, _ := procGetInterfaceDetail.Call(set, uintptr(unsafe.Pointer(&ifc)), uintptr(unsafe.Pointer(&detail[0])), uintptr(len(detail)*2), 0, uintptr(unsafe.Pointer(&info))) //nolint:gosec // Win32 call
		if r == 0 {
			continue
		}
		h, err := syscall.CreateFile(&detail[2], 0, fileShareReadWrite, nil, openExisting, 0, 0)
		if err != nil {
			continue
		}
		num, err := deviceNumber(h)
		_ = syscall.CloseHandle(h)
		if err == nil && num == want {
			return info.devInst, nil
		}
	}
	return 0, errNotUSB
}

// ejectMedia locks and dismounts the volume and ejects the media.
func ejectMedia(letter string) error {
	h, err := openVolume(letter, genericReadWrite)
	if err != nil {
		return fmt.Errorf("open volume: %w", err)
	}
	defer func() { _ = syscall.CloseHandle(h) }()
	if _, err := ioctl(h, fsctlLockVolume, nil, nil); err != nil {
		return fmt.Errorf("volume in use: %w", err)
	}
	if _, err := ioctl(h, fsctlDismountVolume, nil, nil); err != nil {
		return fmt.Errorf("dismount: %w", err)
	}
	if _, err := ioctl(h, ioctlStorageMediaRemoval, []byte{0}, nil); err != nil {
		return fmt.Errorf("allow removal: %w", err)
	}
	if _, err := ioctl(h, ioctlStorageEjectMedia, nil, nil); err != nil && !errors.Is(err, syscall.Errno(errorNotReady)) {
		return fmt.Errorf("eject: %w", err)
	}
	return nil
}
