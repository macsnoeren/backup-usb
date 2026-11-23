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

import "testing"

const lsblkSample = `{"blockdevices": [
 {"name":"sda","path":"/dev/sda","maj:min":"8:0","type":"disk","fstype":null,"label":null,"uuid":null,"partuuid":null,"mountpoints":[null],"rm":false,"hotplug":false,"tran":"sata","serial":"WD-123","pkname":null,
  "children":[
   {"name":"sda1","path":"/dev/sda1","maj:min":"8:1","type":"part","fstype":"vfat","label":null,"uuid":"AB12-CD34","partuuid":"p1","mountpoints":["/boot/efi"],"rm":false,"hotplug":false,"tran":null,"serial":null,"pkname":"sda"},
   {"name":"sda2","path":"/dev/sda2","maj:min":"8:2","type":"part","fstype":"ext4","label":null,"uuid":"root-uuid","partuuid":"p2","mountpoints":["/"],"rm":false,"hotplug":false,"tran":null,"serial":null,"pkname":"sda"},
   {"name":"sda3","path":"/dev/sda3","maj:min":"8:3","type":"part","fstype":"ext4","label":"data","uuid":"data-uuid","partuuid":"p3","mountpoints":["/mnt/data"],"rm":false,"hotplug":false,"tran":null,"serial":null,"pkname":"sda"}
  ]},
 {"name":"sdb","path":"/dev/sdb","maj:min":"8:16","type":"disk","fstype":null,"label":null,"uuid":null,"partuuid":null,"mountpoints":[null],"rm":"1","hotplug":"1","tran":"usb","serial":"575834314131383436","pkname":null,
  "children":[
   {"name":"sdb1","path":"/dev/sdb1","maj:min":"8:17","type":"part","fstype":"exfat","label":"BACKUP1","uuid":"A41C-77E2","partuuid":"6b1f0c3e-01","mountpoints":[null],"rm":"1","hotplug":"1","tran":null,"serial":null,"pkname":"sdb"}
  ]},
 {"name":"sdc","path":"/dev/sdc","maj:min":"8:32","type":"disk","fstype":"swap","mountpoints":["[SWAP]"],"rm":false}
]}`

func TestParseLsblk(t *testing.T) {
	vols, majmin, err := ParseLsblk([]byte(lsblkSample))
	if err != nil {
		t.Fatal(err)
	}
	if len(vols) != 2 {
		t.Fatalf("volumes %+v", vols)
	}
	data, usb := vols[0], vols[1]
	if data.Device != "/dev/sda3" || data.Mount != "/mnt/data" || data.Removable {
		t.Fatalf("data %+v", data)
	}
	if usb.Device != "/dev/sdb1" || usb.Label != "BACKUP1" || usb.VolumeSerial != "A41C-77E2" || usb.PartitionID != "6b1f0c3e-01" ||
		usb.HardwareSerial != "575834314131383436" || !usb.Removable || usb.Mount != "" || usb.FileSystem != "exfat" {
		t.Fatalf("usb %+v", usb)
	}
	if majmin["/dev/sdb1"] != "8:17" || majmin["parent:/dev/sdb1"] != "/dev/sdb" {
		t.Fatalf("majmin %v", majmin)
	}
	if _, _, err := ParseLsblk([]byte("not json")); err == nil {
		t.Fatal("garbage accepted")
	}
}

func TestParseMountinfo(t *testing.T) {
	m := ParseMountinfo([]byte("36 35 8:17 / /media/user/BACKUP\\0401 rw,nosuid - exfat /dev/sdb1 rw\n37 35 8:3 /sub /mnt/bind rw - ext4 /dev/sda3 rw\n38 35 8:3 / /mnt/data rw - ext4 /dev/sda3 rw\n"))
	if m["8:17"] != "/media/user/BACKUP 1" || m["8:3"] != "/mnt/data" {
		t.Fatalf("%v", m)
	}
}

func TestParseUdev(t *testing.T) {
	if got := ParseUdevSerial([]byte("E:ID_SERIAL=SanDisk_Cruzer_575834\nE:ID_SERIAL_SHORT=575834\n")); got != "575834" {
		t.Fatal(got)
	}
	if got := ParseUdevSerial([]byte("E:ID_SERIAL=Generic\n")); got != "Generic" {
		t.Fatal(got)
	}
	if got := ParseUdisksMount("Mounted /dev/sdb1 at /media/user/BACKUP1\n"); got != "/media/user/BACKUP1" {
		t.Fatal(got)
	}
	if ParseUdisksMount("Error") != "" {
		t.Fatal("error output")
	}
}

func FuzzParsers(f *testing.F) {
	f.Add([]byte(lsblkSample))
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _, _ = ParseLsblk(b)
		_ = ParseMountinfo(b)
		_ = ParseUdevSerial(b)
	})
}
