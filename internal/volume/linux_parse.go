package volume

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// LsblkDevice is one entry of "lsblk --json --bytes".
type LsblkDevice struct {
	Name        string        `json:"name"`
	Path        string        `json:"path"`
	MajMin      string        `json:"maj:min"`
	Type        string        `json:"type"`
	FSType      string        `json:"fstype"`
	Label       string        `json:"label"`
	UUID        string        `json:"uuid"`
	PartUUID    string        `json:"partuuid"`
	Mountpoints []string      `json:"mountpoints"`
	RM          jsonBool      `json:"rm"`
	Hotplug     jsonBool      `json:"hotplug"`
	Tran        string        `json:"tran"`
	Serial      string        `json:"serial"`
	PKName      string        `json:"pkname"`
	Children    []LsblkDevice `json:"children"`
}

// jsonBool accepts true/false as well as "1"/"0" (older lsblk).
type jsonBool bool

func (b *jsonBool) UnmarshalJSON(data []byte) error {
	s := strings.Trim(string(data), `"`)
	*b = jsonBool(s == "true" || s == "1")
	return nil
}

// LsblkArgs are the arguments for lsblk.
var LsblkArgs = []string{"--json", "--bytes", "--output", "NAME,PATH,MAJ:MIN,TYPE,FSTYPE,LABEL,UUID,PARTUUID,MOUNTPOINTS,RM,HOTPLUG,TRAN,SERIAL,PKNAME"}

// systemMounts are mount points that mark a system volume.
var systemMounts = map[string]bool{"/": true, "/boot": true, "/boot/efi": true, "/efi": true, "/usr": true, "/var": true, "/home": true, "/opt": true, "/srv": true, "[SWAP]": true}

// ParseLsblk returns the candidate volumes: block devices with a file
// system that are not system volumes. Removable or USB ones are flagged.
// The parent disk's transport and serial are inherited by partitions.
func ParseLsblk(data []byte) ([]Volume, map[string]string, error) {
	var doc struct {
		Blockdevices []LsblkDevice `json:"blockdevices"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, nil, fmt.Errorf("lsblk: %w", err)
	}
	var out []Volume
	majmin := map[string]string{} // device path -> "maj:min"
	parentOf := map[string]string{}
	var walk func(d LsblkDevice, parent *LsblkDevice)
	walk = func(d LsblkDevice, parent *LsblkDevice) {
		tran, rm, hot, serial := d.Tran, bool(d.RM), bool(d.Hotplug), d.Serial
		if parent != nil {
			if tran == "" {
				tran = parent.Tran
			}
			rm = rm || bool(parent.RM)
			hot = hot || bool(parent.Hotplug)
			parentOf[d.Path] = parent.Path
			if serial == "" {
				serial = parent.Serial
			}
		}
		system := false
		var mount string
		for _, m := range d.Mountpoints {
			if systemMounts[m] {
				system = true
			}
			if m != "" && mount == "" {
				mount = m
			}
		}
		if d.FSType != "" && d.FSType != "swap" && d.FSType != "crypto_LUKS" && d.FSType != "LVM2_member" && !system && (d.Type == "part" || d.Type == "disk" || d.Type == "loop") {
			out = append(out, Volume{
				Mount: mount, Device: d.Path, Label: d.Label, FileSystem: d.FSType,
				VolumeSerial: d.UUID, PartitionID: d.PartUUID, HardwareSerial: serial,
				Removable: rm || hot || tran == "usb",
			})
			majmin[d.Path] = d.MajMin
		}
		for _, c := range d.Children {
			c := c
			walk(c, &d)
		}
	}
	for _, d := range doc.Blockdevices {
		walk(d, nil)
	}
	for k, v := range parentOf {
		majmin["parent:"+k] = v
	}
	return out, majmin, nil
}

// unescapeMount decodes the octal escapes (\040 for a space) in
// /proc/self/mountinfo.
func unescapeMount(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+3 < len(s) {
			if n, err := strconv.ParseUint(s[i+1:i+4], 8, 8); err == nil {
				b.WriteByte(byte(n))
				i += 3
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// ParseMountinfo maps "maj:min" to the first mount point of that device.
func ParseMountinfo(data []byte) map[string]string {
	out := map[string]string{}
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		// id parent maj:min root mountpoint options ... - fstype source
		if len(f) < 5 || f[3] != "/" {
			continue
		}
		if _, ok := out[f[2]]; !ok {
			out[f[2]] = unescapeMount(f[4])
		}
	}
	return out
}

// ParseUdevSerial returns ID_SERIAL_SHORT or ID_SERIAL from a
// /run/udev/data file.
func ParseUdevSerial(data []byte) string {
	var serial, short string
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line := sc.Text()
		if v, ok := strings.CutPrefix(line, "E:ID_SERIAL_SHORT="); ok {
			short = v
		} else if v, ok := strings.CutPrefix(line, "E:ID_SERIAL="); ok {
			serial = v
		}
	}
	if short != "" {
		return short
	}
	return serial
}

// ParseUdisksMount reads the mount point from "udisksctl mount" output:
// "Mounted /dev/sdb1 at /media/user/BACKUP1".
func ParseUdisksMount(out string) string {
	_, after, ok := strings.Cut(strings.TrimSpace(out), " at ")
	if !ok {
		return ""
	}
	return strings.TrimSuffix(strings.TrimSpace(after), ".")
}
