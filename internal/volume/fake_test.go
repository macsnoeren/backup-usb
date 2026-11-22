package volume_test

import (
	"context"
	"testing"

	"github.com/macsnoeren/backup-usb/internal/testutil"
	"github.com/macsnoeren/backup-usb/internal/volume"
)

func TestFakeFinderTwoDisks(t *testing.T) {
	set := testutil.FakeDisks(t, "BACKUP1", "BACKUP2")
	set.Update("BACKUP1", func(d *testutil.FakeDisk) { d.FileSystem = "FAT32"; d.FreeBytes = 5 << 30 })
	set.Update("BACKUP2", func(d *testutil.FakeDisk) { d.Health = volume.Unhealthy; d.Unmounted = true; d.HardwareSerial = "" })
	f := set.Finder()
	ctx := context.Background()
	vols, err := f.Volumes(ctx)
	if err != nil || len(vols) != 2 {
		t.Fatal(vols, err)
	}
	b1, b2 := vols[0], vols[1]
	if !volume.IsFAT32(b1.FileSystem) || b1.Mount == "" {
		t.Fatalf("BACKUP1: %+v", b1)
	}
	if free, _, _ := f.Space(ctx, b1); free != 5<<30 {
		t.Fatalf("free %d", free)
	}
	if b2.Mount != "" || f.Health(ctx, b2) != volume.Unhealthy || b2.HardwareSerial != "" {
		t.Fatalf("BACKUP2: %+v", b2)
	}
	m, undo, err := f.Mount(ctx, b2)
	if err != nil || m.Mount == "" || undo() != nil {
		t.Fatalf("mount: %+v %v", m, err)
	}
	if err := f.Eject(ctx, b1); err != nil {
		t.Fatal(err)
	}
	vols, _ = f.Volumes(ctx)
	if len(vols) != 1 || vols[0].Label != "BACKUP2" {
		t.Fatalf("after eject: %+v", vols)
	}
	if volume.ParseHealth("Weird") != volume.Unknown || volume.ParseHealth("Warning") != volume.Warning {
		t.Fatal("ParseHealth")
	}
}

func TestFakeFinderMissingFile(t *testing.T) {
	f := volume.NewFakeFinder(t.TempDir() + "/none.json")
	if _, err := f.Volumes(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}
