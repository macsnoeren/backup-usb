package testutil

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/macsnoeren/backup-usb/internal/volume"
)

// Update is set by "go test ./... -update" and makes Golden rewrite the
// golden files. Every test package imports testutil, so the flag is known
// everywhere.
var Update = flag.Bool("update", false, "rewrite golden files in testdata")

// RepoRoot returns the root of the repository.
func RepoRoot() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		panic("testutil: cannot locate source")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

// Testdata returns the path of name inside the repository testdata folder.
func Testdata(name ...string) string {
	return filepath.Join(append([]string{RepoRoot(), "testdata"}, name...)...)
}

// Golden compares got with testdata/golden/<name>. With -update it writes
// got to the file instead. Line endings are normalised to "\n".
func Golden(t testing.TB, name string, got []byte) {
	t.Helper()
	path := Testdata("golden", name)
	got = bytes.ReplaceAll(got, []byte("\r\n"), []byte("\n"))
	if *Update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("golden file %s: %v (run go test -update)", name, err)
	}
	want = bytes.ReplaceAll(want, []byte("\r\n"), []byte("\n"))
	if !bytes.Equal(got, want) {
		t.Errorf("output differs from golden file %s (run go test -update if intended)\n--- got\n%s\n--- want\n%s", name, got, want)
	}
}

// Clock is a fake clock. The zero value is not useful; use NewClock.
type Clock struct {
	mu  sync.Mutex
	now time.Time
}

// NewClock returns a clock set to t.
func NewClock(t time.Time) *Clock { return &Clock{now: t} }

// Now returns the current fake time.
func (c *Clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// Set sets the fake time.
func (c *Clock) Set(t time.Time) {
	c.mu.Lock()
	c.now = t
	c.mu.Unlock()
}

// Advance moves the fake time forward by d.
func (c *Clock) Advance(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	c.mu.Unlock()
}

// Date is a short helper for a local time at the given day and hour.
func Date(year int, month time.Month, day, hour int) time.Time {
	return time.Date(year, month, day, hour, 0, 0, 0, time.Local)
}

// FakeDisk is one fake disk: a directory with its own identity.
type FakeDisk = volume.FakeDisk

// FakeDiskSet is a set of fake disks backed by a JSON file, as read by
// volume.FakeFinder and by USB_BACKUP_FAKE_DISKS.
type FakeDiskSet struct {
	t    testing.TB
	File string
	mu   sync.Mutex
	data volume.FakeFile
}

// FakeDisks creates one directory per name in t.TempDir() and describes them
// as disks with their own serial numbers, plenty of free space, exFAT and
// health Healthy. The label of each disk is its name.
func FakeDisks(t testing.TB, names ...string) *FakeDiskSet {
	t.Helper()
	root := t.TempDir()
	s := &FakeDiskSet{t: t, File: filepath.Join(root, "fake-disks.json")}
	for i, name := range names {
		mount := filepath.Join(root, "disk-"+name)
		if err := os.MkdirAll(mount, 0o755); err != nil {
			t.Fatal(err)
		}
		s.data.Disks = append(s.data.Disks, volume.FakeDisk{
			Volume: volume.Volume{
				Mount:          mount,
				Device:         fmt.Sprintf("fake%d", i+1),
				Label:          name,
				FileSystem:     "exFAT",
				VolumeSerial:   randomHex(t, 4),
				PartitionID:    "{" + randomHex(t, 16) + "}",
				HardwareSerial: randomHex(t, 9),
				Removable:      true,
				FreeBytes:      64 << 30,
				TotalBytes:     128 << 30,
			},
			Health: volume.Healthy,
		})
	}
	s.Save()
	return s
}

func randomHex(t testing.TB, n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return strings.ToUpper(hex.EncodeToString(b))
}

// Disk returns the fake disk with the given label.
func (s *FakeDiskSet) Disk(label string) *FakeDisk {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.data.Disks {
		if s.data.Disks[i].Label == label {
			return &s.data.Disks[i]
		}
	}
	s.t.Fatalf("no fake disk %q", label)
	return nil
}

// Update changes the disk with the given label and saves the set.
func (s *FakeDiskSet) Update(label string, change func(d *FakeDisk)) {
	d := s.Disk(label)
	s.mu.Lock()
	change(d)
	s.mu.Unlock()
	s.Save()
}

// Remove unplugs the disk with the given label.
func (s *FakeDiskSet) Remove(label string) {
	s.mu.Lock()
	out := s.data.Disks[:0]
	for _, d := range s.data.Disks {
		if d.Label != label {
			out = append(out, d)
		}
	}
	s.data.Disks = out
	s.mu.Unlock()
	s.Save()
}

// Save writes the JSON file.
func (s *FakeDiskSet) Save() {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := json.MarshalIndent(&s.data, "", "  ")
	if err != nil {
		s.t.Fatal(err)
	}
	if err := os.WriteFile(s.File, data, 0o600); err != nil {
		s.t.Fatal(err)
	}
}

// Finder returns a volume.Finder over the fake disks.
func (s *FakeDiskSet) Finder() volume.Finder { return volume.NewFakeFinder(s.File) }

// TestKey returns the content of testdata/keys/<name>.key (an age identity
// file with a comment line naming the public key). Test keys protect nothing.
func TestKey(t testing.TB, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(Testdata("keys", name+".key"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// TestIdentity returns the secret key line of a test key.
func TestIdentity(t testing.TB, name string) string {
	t.Helper()
	for _, line := range strings.Split(string(TestKey(t, name)), "\n") {
		if strings.HasPrefix(line, "AGE-SECRET-KEY-") {
			return line
		}
	}
	t.Fatalf("test key %s has no identity", name)
	return ""
}

// TestRecipient returns the public key of a test key.
func TestRecipient(t testing.TB, name string) string {
	t.Helper()
	for _, line := range strings.Split(string(TestKey(t, name)), "\n") {
		if p, ok := strings.CutPrefix(line, "# public key: "); ok {
			return p
		}
	}
	t.Fatalf("test key %s has no public key", name)
	return ""
}

// TestSigningSeed returns the Ed25519 seed in testdata/keys/<name>.seed.
func TestSigningSeed(t testing.TB, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(Testdata("keys", name+".seed"))
	if err != nil {
		t.Fatal(err)
	}
	seed, err := hex.DecodeString(strings.TrimSpace(string(data)))
	if err != nil {
		t.Fatal(err)
	}
	return seed
}

// WriteFile writes content to dir/rel, creating parent directories.
func WriteFile(t testing.TB, dir, rel, content string) string {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// Tree returns all files under dir as relative slash paths mapped to their
// content; directories map to "<dir>".
func Tree(t testing.TB, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil || rel == "." {
			return err
		}
		rel = filepath.ToSlash(rel)
		if info.IsDir() {
			out[rel] = "<dir>"
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(p)
			out[rel] = "<link " + target + ">"
			return err
		}
		data, err := os.ReadFile(p)
		out[rel] = string(data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}
