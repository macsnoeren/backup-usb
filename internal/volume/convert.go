package volume

import (
	"encoding/binary"
	"fmt"
	"strings"
	"unicode/utf16"
)

// DriveLetters turns a GetLogicalDrives bit mask into "A:" ... "Z:".
func DriveLetters(mask uint32) []string {
	var out []string
	for i := 0; i < 26; i++ {
		if mask&(1<<i) != 0 {
			out = append(out, string(rune('A'+i))+":")
		}
	}
	return out
}

// FormatVolumeSerial formats a volume serial number like "A41C-77E2".
func FormatVolumeSerial(serial uint32) string {
	return fmt.Sprintf("%04X-%04X", serial>>16, serial&0xffff)
}

// FormatGUID formats a GUID from its 16 bytes in Windows memory order
// (Data1..3 little endian) as "{6b1f0c3e-2d4a-4f5b-8e9c-1a2b3c4d5e6f}".
func FormatGUID(b []byte) string {
	if len(b) != 16 {
		return ""
	}
	return fmt.Sprintf("{%08x-%04x-%04x-%x-%x}",
		binary.LittleEndian.Uint32(b[0:4]), binary.LittleEndian.Uint16(b[4:6]), binary.LittleEndian.Uint16(b[6:8]), b[8:10], b[10:16])
}

// FormatMBRPartition identifies an MBR partition by disk signature and
// partition number.
func FormatMBRPartition(signature uint32, partition uint32) string {
	return fmt.Sprintf("MBR:%08X:%d", signature, partition)
}

// UTF16ToString converts a NUL-terminated UTF-16 buffer.
func UTF16ToString(s []uint16) string {
	for i, c := range s {
		if c == 0 {
			s = s[:i]
			break
		}
	}
	return string(utf16.Decode(s))
}

// DescriptorSerial reads the serial number from a STORAGE_DEVICE_DESCRIPTOR
// returned by IOCTL_STORAGE_QUERY_PROPERTY. It returns "" when there is no
// usable serial. The buffer comes from the device and is checked.
func DescriptorSerial(buf []byte) string {
	// Version(4) Size(4) DeviceType(1) DeviceTypeModifier(1)
	// RemovableMedia(1) CommandQueueing(1) VendorIdOffset(4)
	// ProductIdOffset(4) ProductRevisionOffset(4) SerialNumberOffset(4)
	if len(buf) < 28 {
		return ""
	}
	off := binary.LittleEndian.Uint32(buf[24:28])
	if off == 0 || off == 0xffffffff || int(off) >= len(buf) {
		return ""
	}
	end := int(off)
	for end < len(buf) && buf[end] != 0 {
		end++
	}
	s := strings.TrimSpace(string(buf[off:end]))
	for _, r := range s {
		if r < 0x20 || r > 0x7e {
			return ""
		}
	}
	return s
}

// DescriptorRemovable reports the RemovableMedia flag of a
// STORAGE_DEVICE_DESCRIPTOR and the bus type (7 = USB).
func DescriptorRemovable(buf []byte) (removable bool, usb bool) {
	if len(buf) < 32 {
		return false, false
	}
	return buf[10] != 0, binary.LittleEndian.Uint32(buf[28:32]) == 7
}

// ParseHealthOutput reads the HealthStatus printed by PowerShell
// (Get-Disk) or the result line of smartctl -H.
func ParseHealthOutput(out string) Health {
	s := strings.TrimSpace(out)
	switch {
	case strings.EqualFold(s, "Healthy"), strings.Contains(s, "test result: PASSED"), strings.Contains(s, "SMART Health Status: OK"):
		return Healthy
	case strings.EqualFold(s, "Warning"):
		return Warning
	case strings.EqualFold(s, "Unhealthy"), strings.Contains(s, "test result: FAILED"):
		return Unhealthy
	}
	return Unknown
}
