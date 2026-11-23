package cli

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/macsnoeren/backup-usb/internal/app"
	"github.com/macsnoeren/backup-usb/internal/config"
	"github.com/macsnoeren/backup-usb/internal/disk"
	"github.com/macsnoeren/backup-usb/internal/password"
	"github.com/macsnoeren/backup-usb/internal/secmem"
	"github.com/macsnoeren/backup-usb/internal/sign"
	"github.com/macsnoeren/backup-usb/internal/testutil"
	"github.com/macsnoeren/backup-usb/internal/vault"
	"github.com/macsnoeren/backup-usb/internal/week"
)

// world is a complete fake machine: configuration folder, source folders,
// fake disks and a fake clock.
type world struct {
	t       *testing.T
	cfgDir  string
	srcDir  string
	disks   *testutil.FakeDiskSet
	clock   *testutil.Clock
	stdin   string
	ctx     context.Context
	cfgData map[string]any
	bgKey   string
	removed map[string]testutil.FakeDisk
}

// addBack plugs a removed fake disk in again.
func (w *world) addBack(name string) {
	d, ok := w.removed[name]
	if !ok {
		w.t.Fatalf("%s was not removed", name)
	}
	w.disks.Add(d)
}

// newWorld sets up two fake disks and a source folder "Data" with files.
// The clock is in ISO week 47 of 2026 (odd).
func newWorld(t *testing.T) *world {
	w := &world{
		t:      t,
		cfgDir: t.TempDir(),
		srcDir: t.TempDir(),
		disks:  testutil.FakeDisks(t, "BACKUP1", "BACKUP2"),
		clock:  testutil.NewClock(time.Date(2026, 11, 18, 10, 0, 0, 0, time.Local)),
	}
	testutil.WriteFile(t, w.srcDir, "Data/report.docx", strings.Repeat("d", 2000))
	testutil.WriteFile(t, w.srcDir, "Data/notes/todo.txt", "buy milk")
	testutil.WriteFile(t, w.srcDir, "Books/2026/q1.xlsx", "numbers")
	w.cfgData = map[string]any{
		"sources": []map[string]any{
			{"tag": "Data", "path": filepath.Join(w.srcDir, "Data")},
			{"tag": "Books", "path": filepath.Join(w.srcDir, "Books")},
		},
	}
	w.writeConfig()
	return w
}

func (w *world) writeConfig() {
	data, err := json.MarshalIndent(w.cfgData, "", "  ")
	if err != nil {
		w.t.Fatal(err)
	}
	testutil.WriteFile(w.t, w.cfgDir, config.FileName, string(data))
	w.resign()
}

// resign signs the settings and registration list with the operator key,
// like the CLI does, once keys exist (tests write these files directly).
func (w *world) resign() {
	if _, err := os.Stat(filepath.Join(w.cfgDir, config.SigningPubFile)); err != nil {
		return
	}
	v, err := vault.Open(w.vaultPath(), secmemFrom(testPassword))
	if err != nil {
		w.t.Fatal(err)
	}
	defer v.Close()
	key := v.SigningKey()
	defer key.Wipe()
	for _, f := range []struct{ name, purpose string }{{config.FileName, sign.PurposeConfig}, {config.DefaultDisksFile, sign.PurposeRegistry}} {
		p := filepath.Join(w.cfgDir, f.name)
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		sig, err := sign.SigFile(key.Bytes(), f.purpose, data)
		if err != nil {
			w.t.Fatal(err)
		}
		testutil.WriteFile(w.t, w.cfgDir, f.name+".sig", string(sig))
	}
}

// set changes one setting.
func (w *world) set(key string, v any) {
	w.cfgData[key] = v
	w.writeConfig()
}

// oldWeeks creates n week folders before the current week on a disk.
func (w *world) oldWeeks(name string, n int) []string {
	var rels []string
	per := week.For(w.clock.Now(), week.ISOWeek)
	pat, _ := week.ParsePattern(`{year}\{week}`, week.ISOWeek)
	for i := 0; i < n; i++ {
		per = week.For(per.Start.Add(-time.Hour), week.ISOWeek)
		rel := pat.Folder(per)
		testutil.WriteFile(w.t, w.disks.Disk(name).Mount, "Backup/"+rel+"/Data.zip", "old")
		rels = append(rels, rel)
	}
	return rels
}

func uuid(t *testing.T) string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// enrollUnsigned registers fake disks without signatures, as before
// phase 6. weeks maps disk name to "odd" or "even".
func (w *world) enrollUnsigned(weeks map[string]string) {
	reg := &disk.Registry{Version: 1}
	for _, name := range []string{"BACKUP1", "BACKUP2"} {
		parity, ok := weeks[name]
		if !ok {
			continue
		}
		d := w.disks.Disk(name)
		r := disk.Registration{Name: name, Weeks: parity, ID: uuid(w.t), VolumeSerial: d.VolumeSerial, PartitionID: d.PartitionID, HardwareSerial: d.HardwareSerial, Enrolled: "2026-11-01"}
		reg.Disks = append(reg.Disks, r)
		id := disk.IDFile{Version: 1, ID: r.ID, Name: name, VolumeSerial: r.VolumeSerial, PartitionID: r.PartitionID, HardwareSerial: r.HardwareSerial, Enrolled: r.Enrolled}
		data, _ := json.Marshal(id)
		testutil.WriteFile(w.t, d.Mount, "Backup/disk-id.json", string(data))
	}
	data, err := reg.Marshal()
	if err != nil {
		w.t.Fatal(err)
	}
	testutil.WriteFile(w.t, w.cfgDir, config.DefaultDisksFile, string(data))
	w.resign()
}

// unplug removes a disk and remembers it for addBack.
func (w *world) unplug(name string) {
	if w.removed == nil {
		w.removed = map[string]testutil.FakeDisk{}
	}
	w.removed[name] = *w.disks.Disk(name)
	w.disks.Remove(name)
}

func (w *world) env() *Env {
	return &Env{
		Stdin:     strings.NewReader(w.stdin),
		Now:       w.clock.Now,
		Finder:    w.disks.Finder(),
		ConfigDir: w.cfgDir,
		Getenv:    func(string) string { return "" },
		Context:   w.ctx,
		LockPath:  filepath.Join(w.cfgDir, "test.lock"),
		Sleep: func(_ context.Context, d time.Duration) error {
			w.clock.Advance(d)
			return nil
		},
	}
}

// run runs the CLI in-process.
func (w *world) run(args ...string) (int, string, string) {
	w.t.Helper()
	return run(w.t, w.env(), args...)
}

// mustRun runs and fails the test on an unexpected exit code.
func (w *world) mustRun(code int, args ...string) (string, string) {
	w.t.Helper()
	got, out, errOut := w.run(args...)
	if got != code {
		w.t.Fatalf("%v: exit %d, want %d\nstdout:\n%s\nstderr:\n%s", args, got, code, out, errOut)
	}
	return out, errOut
}

// diskFile returns the path of rel on a fake disk.
func (w *world) diskFile(name, rel string) string {
	return filepath.Join(w.disks.Disk(name).Mount, filepath.FromSlash(rel))
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func secmemFrom(s string) *secmem.Buffer { return secmem.From([]byte(s)) }

func testIdentity(t *testing.T, name string) string { return testutil.TestIdentity(t, name) }

func hashOf(p string) (string, int64, error) {
	root, err := os.OpenRoot(filepath.Dir(p))
	if err != nil {
		return "", 0, err
	}
	defer root.Close()
	return app.HashFile(context.Background(), root, filepath.Base(p))
}

// diskSerial returns a field of an ID file.
func (w *world) diskSerial(idFile []byte, field string) string {
	var m map[string]any
	if err := json.Unmarshal(idFile, &m); err != nil {
		w.t.Fatal(err)
	}
	s, _ := m[field].(string)
	return s
}

func isWindows() bool { return runtime.GOOS == "windows" }

// passwordReader is a console that answers with pw.
func passwordReader(pw string) password.Source {
	return password.Reader{R: strings.NewReader(pw + "\n")}
}
