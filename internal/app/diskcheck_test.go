package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/macsnoeren/backup-usb/internal/testutil"
	"github.com/macsnoeren/backup-usb/internal/volume"
)

func TestDiskChecks(t *testing.T) {
	const gb = 1 << 30
	tests := []struct {
		name   string
		fs     string
		free   uint64
		need   SpaceNeed
		key    string
		failed bool
	}{
		{"enough space", "exFAT", 10 * gb, SpaceNeed{SourceBytes: []uint64{2 * gb, 3 * gb}, MinFreePercent: 5}, "", false},
		{"too little space", "exFAT", 5 * gb, SpaceNeed{SourceBytes: []uint64{2 * gb, 3 * gb}, MinFreePercent: 5}, "err.space", true},
		{"margin counts", "NTFS", 5*gb + 50<<20, SpaceNeed{SourceBytes: []uint64{5 * gb}}, "err.space", true},
		{"fat32 small zip", "FAT32", 10 * gb, SpaceNeed{SourceBytes: []uint64{3 * gb}}, "", false},
		{"fat32 big zip", "FAT32", 20 * gb, SpaceNeed{SourceBytes: []uint64{5 * gb}}, "err.fat32", true},
		{"vfat big zip", "vfat", 20 * gb, SpaceNeed{SourceBytes: []uint64{1 * gb, 5 * gb}}, "err.fat32", true},
		{"exfat big zip", "exFAT", 20 * gb, SpaceNeed{SourceBytes: []uint64{5 * gb}}, "", false},
	}
	for _, tt := range tests {
		disks := testutil.FakeDisks(t, "BACKUP1")
		disks.Update("BACKUP1", func(d *testutil.FakeDisk) { d.FileSystem = tt.fs; d.FreeBytes = tt.free })
		v := disks.Disk("BACKUP1").Volume
		err := CheckDisk(context.Background(), disks.Finder(), v, filepath.Join(v.Mount, "Backup"), tt.need)
		if (err != nil) != tt.failed {
			t.Errorf("%s: err = %v", tt.name, err)
			continue
		}
		if err != nil {
			var e *Error
			if !errors.As(err, &e) || e.Kind != KindDiskCheck || e.Key != tt.key {
				t.Errorf("%s: kind %v key %s", tt.name, e.Kind, e.Key)
			}
		}
	}
}

func TestNotWritable(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs a read-only folder")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o700) })
	err := CheckWritable(filepath.Join(dir, "Backup"))
	if KindOf(err) != KindDiskCheck {
		t.Fatalf("err = %v", err)
	}
	if err := CheckWritable(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	_ = volume.Healthy
}

func TestHumanBytes(t *testing.T) {
	if humanBytes(512) != "512 B" || humanBytes(1536) != "1.5 KiB" || humanBytes(5<<30) != "5.0 GiB" {
		t.Fatal(humanBytes(1536), humanBytes(5<<30))
	}
}
