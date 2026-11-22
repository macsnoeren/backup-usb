package testutil

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"filippo.io/age"
)

func TestFakeDisks(t *testing.T) {
	set := FakeDisks(t, "BACKUP1", "BACKUP2")
	f := set.Finder()
	vols, err := f.Volumes(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(vols) != 2 {
		t.Fatalf("got %d volumes", len(vols))
	}
	if vols[0].VolumeSerial == vols[1].VolumeSerial {
		t.Fatal("fake disks share a serial number")
	}
	if st, err := os.Stat(vols[0].Mount); err != nil || !st.IsDir() {
		t.Fatalf("mount dir: %v", err)
	}
	set.Update("BACKUP2", func(d *FakeDisk) { d.FreeBytes = 1000; d.Health = "Warning" })
	free, _, err := f.Space(context.Background(), vols[1])
	if err != nil || free != 1000 {
		t.Fatalf("free = %d, %v", free, err)
	}
	if h := f.Health(context.Background(), vols[1]); h != "Warning" {
		t.Fatalf("health = %s", h)
	}
	set.Remove("BACKUP1")
	vols, _ = f.Volumes(context.Background())
	if len(vols) != 1 || vols[0].Label != "BACKUP2" {
		t.Fatalf("after remove: %+v", vols)
	}
}

func TestClock(t *testing.T) {
	c := NewClock(Date(2026, time.December, 28, 12))
	c.Advance(24 * time.Hour)
	if got := c.Now(); got.Day() != 29 {
		t.Fatalf("got %v", got)
	}
}

func TestGolden(t *testing.T) {
	Golden(t, "testutil-example.txt", []byte("golden example\n"))
}

func TestKeys(t *testing.T) {
	for _, name := range []string{"operational", "breakglass", "operational2"} {
		id, err := age.ParseHybridIdentity(TestIdentity(t, name))
		if err != nil {
			t.Fatal(err)
		}
		if got := id.Recipient().String(); got != TestRecipient(t, name) {
			t.Fatalf("%s: recipient mismatch", name)
		}
		if !strings.HasPrefix(TestRecipient(t, name), "age1pq1") {
			t.Fatalf("%s is not post-quantum", name)
		}
	}
	if len(TestSigningSeed(t, "signing")) != 32 {
		t.Fatal("bad seed")
	}
}

func TestTreeAndWriteFile(t *testing.T) {
	dir := t.TempDir()
	WriteFile(t, dir, "a/b.txt", "x")
	got := Tree(t, dir)
	if got["a"] != "<dir>" || got["a/b.txt"] != "x" || len(got) != 2 {
		t.Fatalf("tree: %v", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "a", "b.txt")); err != nil {
		t.Fatal(err)
	}
}
