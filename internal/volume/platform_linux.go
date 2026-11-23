package volume

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"syscall"
	"time"
)

// toolDirs are searched for external programs. PATH is not used, so a
// planted program earlier in PATH is never started.
var toolDirs = []string{"/usr/sbin", "/usr/bin", "/sbin", "/bin", "/usr/local/sbin", "/usr/local/bin"}

// Tool paths; tests may replace them.
var (
	lsblkPath     = findTool("lsblk")
	udisksctlPath = findTool("udisksctl")
	mountPath     = findTool("mount")
	umountPath    = findTool("umount")
	smartctlPath  = findTool("smartctl")
	// udevDataDir and mountinfoPath are files read for serial numbers and
	// mount points.
	udevDataDir   = "/run/udev/data"
	mountinfoPath = "/proc/self/mountinfo"
	// mountBase is where root mounts volumes itself.
	mountBase = "/run/usb-backup"
)

func findTool(name string) string {
	for _, d := range toolDirs {
		p := filepath.Join(d, name)
		if st, err := os.Stat(p); err == nil && st.Mode().IsRegular() && st.Mode()&0o111 != 0 {
			return p
		}
	}
	return ""
}

type linuxFinder struct{}

// NewPlatformFinder returns the finder for this platform.
func NewPlatformFinder() Finder { return linuxFinder{} }

func run(ctx context.Context, timeout time.Duration, name string, args ...string) ([]byte, error) {
	if name == "" {
		return nil, errors.New("program not installed")
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // full path from a fixed list, arguments as a list
	cmd.Env = []string{"LC_ALL=C", "PATH=/usr/sbin:/usr/bin:/sbin:/bin"}
	return cmd.Output()
}

// Volumes implements Finder with lsblk, /proc/self/mountinfo and udev.
func (linuxFinder) Volumes(ctx context.Context) ([]Volume, error) {
	out, err := run(ctx, 15*time.Second, lsblkPath, LsblkArgs...)
	if err != nil {
		return nil, fmt.Errorf("lsblk: %w", err)
	}
	vols, majmin, err := ParseLsblk(out)
	if err != nil {
		return nil, err
	}
	var mounts map[string]string
	if data, err := os.ReadFile(mountinfoPath); err == nil {
		mounts = ParseMountinfo(data)
	}
	for i := range vols {
		v := &vols[i]
		if mm := majmin[v.Device]; mm != "" && mounts[mm] != "" {
			v.Mount = mounts[mm]
		}
		if v.HardwareSerial == "" {
			v.HardwareSerial = udevSerial(majmin[v.Device])
		}
		if v.Mount != "" {
			v.FreeBytes, v.TotalBytes, _ = statfs(v.Mount)
		}
	}
	return vols, nil
}

var majminRe = regexp.MustCompile(`^\d+:\d+$`)

func udevSerial(mm string) string {
	if !majminRe.MatchString(mm) {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(udevDataDir, "b"+mm))
	if err != nil {
		return ""
	}
	return ParseUdevSerial(data)
}

func statfs(path string) (free, total uint64, err error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, 0, err
	}
	bs := uint64(max(st.Bsize, 0))
	return st.Bavail * bs, st.Blocks * bs, nil
}

// Space implements Finder.
func (linuxFinder) Space(_ context.Context, v Volume) (uint64, uint64, error) {
	if v.Mount == "" {
		return 0, 0, ErrNotMounted
	}
	return statfs(v.Mount)
}

var devRe = regexp.MustCompile(`^/dev/[A-Za-z0-9._-]+$`)

// diskOf returns the whole-disk device of a partition (/dev/sdb1 ->
// /dev/sdb) using sysfs.
func diskOf(dev string) string {
	name := filepath.Base(dev)
	link, err := filepath.EvalSymlinks(filepath.Join("/sys/class/block", name))
	if err != nil {
		return dev
	}
	if _, err := os.Stat(filepath.Join(link, "partition")); err == nil {
		return "/dev/" + filepath.Base(filepath.Dir(link))
	}
	return dev
}

// Health runs "smartctl -H" on the disk if smartctl is installed.
func (linuxFinder) Health(ctx context.Context, v Volume) Health {
	if !devRe.MatchString(v.Device) || smartctlPath == "" {
		return Unknown
	}
	out, _ := run(ctx, 20*time.Second, smartctlPath, "-H", diskOf(v.Device))
	return ParseHealthOutput(string(out))
}

var uuidSafe = regexp.MustCompile(`^[A-Za-z0-9-]{1,64}$`)

// mountableFS are the file systems that are mounted at all; the type is
// always given, so the kernel never has to guess from untrusted data.
var mountableFS = map[string]bool{"vfat": true, "exfat": true, "ntfs": true, "ntfs3": true, "ext2": true, "ext3": true, "ext4": true, "btrfs": true, "xfs": true}

// Mount mounts a volume that is not mounted: with udisksctl (no root
// needed), or as root to /run/usb-backup/<uuid> with nosuid,nodev,noexec.
// The returned function unmounts it again.
func (linuxFinder) Mount(ctx context.Context, v Volume) (Volume, func() error, error) {
	nothing := func() error { return nil }
	if v.Mount != "" {
		return v, nothing, nil
	}
	if !devRe.MatchString(v.Device) {
		return v, nothing, fmt.Errorf("invalid device %q", v.Device)
	}
	if !mountableFS[v.FileSystem] {
		return v, nothing, fmt.Errorf("file system %q is not mounted by usb-backup", v.FileSystem)
	}
	if os.Geteuid() == 0 && mountPath != "" && uuidSafe.MatchString(v.VolumeSerial) {
		dir := filepath.Join(mountBase, v.VolumeSerial)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return v, nothing, err
		}
		if _, err := run(ctx, 60*time.Second, mountPath, "-t", v.FileSystem, "-o", "nosuid,nodev,noexec", "--", v.Device, dir); err != nil {
			return v, nothing, fmt.Errorf("mount: %w", err)
		}
		v.Mount = dir
		return v, func() error {
			_, err := run(context.Background(), 60*time.Second, umountPath, "--", dir)
			if err == nil {
				_ = os.Remove(dir)
			}
			return err
		}, nil
	}
	if _, err := run(ctx, 60*time.Second, udisksctlPath, "mount", "--no-user-interaction", "-t", v.FileSystem, "-b", v.Device); err != nil {
		return v, nothing, fmt.Errorf("udisksctl mount: %w", err)
	}
	// Take the mount point from the kernel, not from udisksctl's text (a
	// label like "BACKUP." would be cut wrongly).
	vols, err := linuxFinder{}.Volumes(ctx)
	if err != nil {
		return v, nothing, err
	}
	for _, x := range vols {
		if x.Device == v.Device && x.Mount != "" && filepath.IsAbs(x.Mount) {
			v.Mount = x.Mount
		}
	}
	if v.Mount == "" {
		return v, nothing, errors.New("mount point not found after udisksctl mount")
	}
	return v, func() error {
		_, err := run(context.Background(), 60*time.Second, udisksctlPath, "unmount", "--no-user-interaction", "-b", v.Device)
		return err
	}, nil
}

// Eject unmounts the volume and powers the disk off with udisksctl.
func (linuxFinder) Eject(ctx context.Context, v Volume) error {
	if !devRe.MatchString(v.Device) {
		return fmt.Errorf("invalid device %q", v.Device)
	}
	if v.Mount != "" {
		if _, err := run(ctx, 60*time.Second, udisksctlPath, "unmount", "--no-user-interaction", "-b", v.Device); err != nil {
			if _, err2 := run(ctx, 60*time.Second, umountPath, "--", v.Mount); err2 != nil {
				return fmt.Errorf("unmount: %w", errors.Join(err, err2))
			}
		}
	}
	if _, err := run(ctx, 60*time.Second, udisksctlPath, "power-off", "--no-user-interaction", "-b", diskOf(v.Device)); err != nil {
		return fmt.Errorf("power off: %w", err)
	}
	return nil
}
