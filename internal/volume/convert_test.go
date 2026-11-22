package volume

import (
	"encoding/binary"
	"reflect"
	"testing"
)

func TestConversions(t *testing.T) {
	if got := DriveLetters(1<<4 | 1<<2 | 1<<25); !reflect.DeepEqual(got, []string{"C:", "E:", "Z:"}) {
		t.Fatal(got)
	}
	if got := FormatVolumeSerial(0xA41C77E2); got != "A41C-77E2" {
		t.Fatal(got)
	}
	g := []byte{0x3e, 0x0c, 0x1f, 0x6b, 0x4a, 0x2d, 0x5b, 0x4f, 0x8e, 0x9c, 0x1a, 0x2b, 0x3c, 0x4d, 0x5e, 0x6f}
	if got := FormatGUID(g); got != "{6b1f0c3e-2d4a-4f5b-8e9c-1a2b3c4d5e6f}" {
		t.Fatal(got)
	}
	if FormatGUID(g[:3]) != "" || FormatMBRPartition(0xdeadbeef, 1) != "MBR:DEADBEEF:1" {
		t.Fatal("mbr")
	}
	if got := UTF16ToString([]uint16{'N', 'T', 'F', 'S', 0, 'x'}); got != "NTFS" {
		t.Fatal(got)
	}
}

func descriptor(serial string, removable bool, bus uint32) []byte {
	buf := make([]byte, 40, 64)
	binary.LittleEndian.PutUint32(buf[24:], 40)
	if removable {
		buf[10] = 1
	}
	binary.LittleEndian.PutUint32(buf[28:], bus)
	return append(append(buf, serial...), 0)
}

func TestDescriptor(t *testing.T) {
	if got := DescriptorSerial(descriptor("  575834314131383436 ", true, 7)); got != "575834314131383436" {
		t.Fatal(got)
	}
	r, usb := DescriptorRemovable(descriptor("x", true, 7))
	if !r || !usb {
		t.Fatal("flags")
	}
	bad := descriptor("x", false, 1)
	binary.LittleEndian.PutUint32(bad[24:], 9999)
	if DescriptorSerial(bad) != "" || DescriptorSerial(nil) != "" || DescriptorSerial(descriptor("\x01\x02", false, 0)) != "" {
		t.Fatal("bad descriptors")
	}
}

func TestParseHealthOutput(t *testing.T) {
	for in, want := range map[string]Health{
		"Healthy\r\n": Healthy, "Warning": Warning, "Unhealthy": Unhealthy, "": Unknown, "Error: no disk": Unknown,
		"SMART overall-health self-assessment test result: PASSED":  Healthy,
		"SMART overall-health self-assessment test result: FAILED!": Unhealthy,
	} {
		if got := ParseHealthOutput(in); got != want {
			t.Errorf("%q: %s", in, got)
		}
	}
}

func FuzzDescriptorSerial(f *testing.F) {
	f.Add(descriptor("abc", true, 7))
	f.Fuzz(func(t *testing.T, b []byte) { _ = DescriptorSerial(b); _, _ = DescriptorRemovable(b) })
}
