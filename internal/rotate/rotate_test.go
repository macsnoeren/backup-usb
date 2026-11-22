package rotate

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/macsnoeren/backup-usb/internal/testutil"
	"github.com/macsnoeren/backup-usb/internal/week"
)

func pattern(t *testing.T, p string) *week.Pattern {
	pat, err := week.ParsePattern(p, week.ISOWeek)
	if err != nil {
		t.Fatal(err)
	}
	return pat
}

// makeWeeks creates n weekly folders ending in the week of last.
func makeWeeks(t *testing.T, dir string, last time.Time, n int, pat *week.Pattern) []string {
	var rels []string
	per := week.For(last, week.ISOWeek)
	for i := 0; i < n; i++ {
		rel := pat.Folder(per)
		testutil.WriteFile(t, dir, rel+"/Data.zip", "zip")
		rels = append([]string{rel}, rels...)
		per = week.For(per.Start.Add(-24*time.Hour), week.ISOWeek)
	}
	return rels
}

func openRoot(t *testing.T, dir string) *os.Root {
	r, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	return r
}

func TestRotateAcrossYearBoundary(t *testing.T) {
	dir := t.TempDir()
	pat := pattern(t, `{year}\{week}`)
	now := time.Date(2027, 1, 20, 12, 0, 0, 0, time.Local) // 2027-W03
	rels := makeWeeks(t, dir, now, 27, pat)
	if rels[0] != "2026/30" || rels[26] != "2027/03" {
		t.Fatalf("setup: %s .. %s", rels[0], rels[26])
	}
	// Things that look similar but must never be touched.
	testutil.WriteFile(t, dir, "foto's/01/x.jpg", "photo")
	testutil.WriteFile(t, dir, "2026/foto's/y.jpg", "photo")
	testutil.WriteFile(t, dir, "2026/5/z", "not zero padded")
	testutil.WriteFile(t, dir, "2026/54/z", "no such week")
	testutil.WriteFile(t, dir, "disk-id.json", "{}")
	root := openRoot(t, dir)
	folders, err := Find(root, []*week.Pattern{pat}, week.ISOWeek)
	if err != nil || len(folders) != 27 {
		t.Fatalf("found %d: %v", len(folders), err)
	}
	del := Plan(folders, Policy{KeepPerDisk: 25}, now, "2027/03")
	if len(del) != 2 || del[0].Rel != "2026/30" || del[1].Rel != "2026/31" {
		t.Fatalf("plan: %+v", del)
	}
	if _, err := Delete(root, del); err != nil {
		t.Fatal(err)
	}
	left, _ := Find(root, []*week.Pattern{pat}, week.ISOWeek)
	if len(left) != 25 || left[0].Rel != "2026/32" {
		t.Fatalf("left %d, oldest %s", len(left), left[0].Rel)
	}
	tree := testutil.Tree(t, dir)
	for _, keep := range []string{"foto's/01/x.jpg", "2026/foto's/y.jpg", "2026/5/z", "2026/54/z", "disk-id.json"} {
		if _, ok := tree[keep]; !ok {
			t.Errorf("%s was removed", keep)
		}
	}
}

func TestEmptyYearFolderRemoved(t *testing.T) {
	dir := t.TempDir()
	pat := pattern(t, `{year}\{week}`)
	now := time.Date(2027, 1, 20, 12, 0, 0, 0, time.Local)
	makeWeeks(t, dir, now, 5, pat) // 2026/52, 2026/53, 2027/01..03
	root := openRoot(t, dir)
	folders, _ := Find(root, []*week.Pattern{pat}, week.ISOWeek)
	if _, err := Delete(root, Plan(folders, Policy{KeepPerDisk: 3}, now)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "2026")); !os.IsNotExist(err) {
		t.Fatal("empty year folder still there")
	}
}

func TestKeepZeroAndMinDays(t *testing.T) {
	dir := t.TempDir()
	pat := pattern(t, `{year}-{week}`)
	now := time.Date(2026, 11, 22, 12, 0, 0, 0, time.Local)
	makeWeeks(t, dir, now, 10, pat)
	root := openRoot(t, dir)
	folders, _ := Find(root, []*week.Pattern{pat}, week.ISOWeek)
	if len(folders) != 10 {
		t.Fatalf("found %d", len(folders))
	}
	if del := Plan(folders, Policy{KeepPerDisk: 0}, now); len(del) != 0 {
		t.Fatalf("keepPerDisk 0 deleted %v", del)
	}
	// Keep 2, but nothing younger than 50 days: only folders older than 50
	// days go.
	del := Plan(folders, Policy{KeepPerDisk: 2, KeepMinDays: 50}, now)
	for _, f := range del {
		if now.Sub(f.Period.Start) < 50*24*time.Hour {
			t.Fatalf("deleted young folder %s", f.Rel)
		}
	}
	if len(del) != 3 {
		t.Fatalf("deleted %d", len(del))
	}
}

func TestSymlinkFoldersAreIgnored(t *testing.T) {
	outside := t.TempDir()
	testutil.WriteFile(t, outside, "precious.txt", "x")
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "2020"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "2020", "01")); err != nil {
		t.Skip("no symlinks:", err)
	}
	pat := pattern(t, `{year}\{week}`)
	makeWeeks(t, dir, time.Date(2026, 11, 22, 12, 0, 0, 0, time.Local), 3, pat)
	root := openRoot(t, dir)
	folders, _ := Find(root, []*week.Pattern{pat}, week.ISOWeek)
	for _, f := range folders {
		if f.Rel == "2020/01" {
			t.Fatal("symlink counted as a week folder")
		}
	}
	// Even a forged plan cannot delete through the link.
	if _, err := Delete(root, []Folder{{Rel: "2020/01"}}); err == nil {
		t.Fatal("deleting a symlink folder succeeded")
	}
	if _, err := Delete(root, []Folder{{Rel: "../x"}}); err == nil {
		t.Fatal("deleting outside the root succeeded")
	}
	if _, err := os.Stat(filepath.Join(outside, "precious.txt")); err != nil {
		t.Fatal("file outside removed")
	}
}

func TestFutureFoldersDoNotCount(t *testing.T) {
	dir := t.TempDir()
	pat := pattern(t, `{year}\{week}`)
	now := time.Date(2026, 11, 22, 12, 0, 0, 0, time.Local)
	makeWeeks(t, dir, now, 3, pat)
	testutil.WriteFile(t, dir, "2099/01/Data.zip", "planted")
	testutil.WriteFile(t, dir, "2098/01/Data.zip", "planted")
	root := openRoot(t, dir)
	folders, _ := Find(root, []*week.Pattern{pat}, week.ISOWeek)
	if len(folders) != 5 {
		t.Fatal(len(folders))
	}
	if del := Plan(folders, Policy{KeepPerDisk: 3}, now); len(del) != 0 {
		t.Fatalf("future folders pushed real backups out: %+v", del)
	}
	if got := NotFuture(folders, now); len(got) != 3 {
		t.Fatal(len(got))
	}
}

func TestMixedPatterns(t *testing.T) {
	dir := t.TempDir()
	a, b := pattern(t, `{year}\{week}`), pattern(t, `{year}-W{week}`)
	now := time.Date(2026, 11, 22, 12, 0, 0, 0, time.Local)
	makeWeeks(t, dir, week.For(now, week.ISOWeek).Start.AddDate(0, 0, -21), 3, a)
	makeWeeks(t, dir, now, 3, b)
	root := openRoot(t, dir)
	folders, _ := Find(root, []*week.Pattern{a, b}, week.ISOWeek)
	if len(folders) != 6 || folders[0].Rel != fmt.Sprintf("2026/%02d", 47-5) {
		t.Fatalf("folders %+v", folders)
	}
}
