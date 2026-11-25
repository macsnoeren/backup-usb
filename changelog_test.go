// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Maurice Snoeren
//
// This file is part of USB Backup.
//
// USB Backup is free software: you can redistribute it and/or modify it under
// the terms of the GNU General Public License as published by the Free Software
// Foundation, either version 3 of the License, or (at your option) any later
// version.
//
// USB Backup is distributed in the hope that it will be useful, but WITHOUT ANY
// WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A
// PARTICULAR PURPOSE. See the GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License along with
// USB Backup. If not, see <https://www.gnu.org/licenses/>.

package backupusb_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

var versionHeading = regexp.MustCompile(`^## (\d+)\.(\d+)\.(\d+)$`)

// TestChangelog checks CHANGELOG.md: an Unreleased section first, then
// versions, newest first, and scripts/release-notes.sh can extract every one
// of them.
func TestChangelog(t *testing.T) {
	b, err := os.ReadFile("CHANGELOG.md")
	if err != nil {
		t.Fatal(err)
	}
	var versions []string
	var prev [3]int
	sawUnreleased := false
	for _, line := range strings.Split(string(b), "\n") {
		switch {
		case line == "## Unreleased":
			if len(versions) > 0 {
				t.Error("## Unreleased must come before the versions")
			}
			sawUnreleased = true
		case strings.HasPrefix(line, "## "):
			m := versionHeading.FindStringSubmatch(line)
			if m == nil {
				t.Errorf("heading %q is not ## X.Y.Z", line)
				continue
			}
			var v [3]int
			for i := range v {
				v[i], _ = strconv.Atoi(m[i+1])
			}
			if len(versions) > 0 && !less(v, prev) {
				t.Errorf("version %s is not lower than the one above it", line)
			}
			prev = v
			versions = append(versions, m[1]+"."+m[2]+"."+m[3])
		}
	}
	if !sawUnreleased {
		t.Error("no ## Unreleased section")
	}
	if len(versions) == 0 {
		t.Fatal("no versions in CHANGELOG.md")
	}

	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no sh: release-notes.sh not tested")
	}
	for _, v := range versions {
		out, err := exec.Command(sh, "scripts/release-notes.sh", "v"+v).CombinedOutput()
		if err != nil || len(strings.TrimSpace(string(out))) == 0 {
			t.Errorf("release-notes.sh v%s: %v\n%s", v, err, out)
		}
	}
	if out, err := exec.Command(sh, "scripts/release-notes.sh", "v999.0.0").CombinedOutput(); err == nil {
		t.Errorf("release-notes.sh accepted a version without a section:\n%s", out)
	}
}

// TestCheckRelease checks which tags scripts/check-release.sh accepts.
func TestCheckRelease(t *testing.T) {
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no sh")
	}
	const old = "## 1.2.0\n\n- First.\n"
	const ready = "# C\n\n## Unreleased\n\n## 1.3.0\n\n### Added\n\n- New.\n\n" + old
	for _, tc := range []struct {
		name, tag, changelog string
		ok                   bool
	}{
		{"newest version", "v1.3.0", ready, true},
		{"headings only under Unreleased", "v1.3.0", strings.Replace(ready, "Unreleased\n", "Unreleased\n\n### Added\n", 1), true},
		{"changes left under Unreleased", "v1.3.0", strings.Replace(ready, "Unreleased\n", "Unreleased\n\n- Forgotten.\n", 1), false},
		{"older version", "v1.2.0", ready, false},
		{"not in the changelog", "v1.4.0", ready, false},
		{"with a date", "v1.3.0", strings.Replace(ready, "## 1.3.0", "## 1.3.0 - 2026-11-20", 1), false},
		{"in brackets", "v1.3.0", strings.Replace(ready, "## 1.3.0", "## [1.3.0]", 1), false},
		{"newer version in brackets", "v1.2.0", strings.Replace(ready, "## 1.3.0", "## [1.3.0]", 1), false},
		{"longer version", "v1.3.0", strings.Replace(ready, "## 1.3.0", "## 1.3.01", 1), false},
		{"Unreleased in brackets", "v1.3.0", strings.Replace(ready, "## Unreleased", "## [Unreleased]", 1), false},
		{"empty section", "v1.3.0", "## Unreleased\n\n## 1.3.0\n\n" + old, false},
		{"pre-release", "v1.4.0-rc1", strings.Replace(ready, "Unreleased\n", "Unreleased\n\n- Next.\n", 1), true},
		{"pre-release without changes", "v1.4.0-rc1", ready, false},
		{"no v", "1.3.0", ready, false},
		{"leading zero", "v1.03.0", ready, false},
		{"too short", "v1.3", ready, false},
		{"branch", "main", ready, false},
		{"empty", "", ready, false},
		{"shell", "v1.3.0;id", ready, false},
		{"substitution", "v1.3.0$(id)", ready, false},
		{"second line", "v1.3.0\nx", ready, false},
		{"option", "-v1.3.0", ready, false},
		{"pattern", "v1.3.*", ready, false},
	} {
		file := filepath.Join(t.TempDir(), "CHANGELOG.md")
		if err := os.WriteFile(file, []byte(tc.changelog), 0o600); err != nil {
			t.Fatal(err)
		}
		out, err := exec.Command(sh, "scripts/check-release.sh", tc.tag, file).CombinedOutput()
		if (err == nil) != tc.ok {
			t.Errorf("%s: %v\n%s", tc.name, err, out)
		}
		if err != nil && len(out) == 0 {
			t.Errorf("%s: no explanation", tc.name)
		}
	}
}

// TestPlanRelease checks when the release workflow releases, against a local
// remote and a fake gh.
func TestPlanRelease(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the workflow runs this script on Linux")
	}
	for _, tool := range []string{"sh", "git"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skip("no " + tool)
		}
	}
	dir := t.TempDir()
	work, bin := filepath.Join(dir, "work"), filepath.Join(dir, "bin")
	for _, d := range []string{work, bin} {
		if err := os.Mkdir(d, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	// RELEASED is the tag that has a release, BODY its text.
	const gh = "#!/bin/sh\n[ \"$1 $2 $3\" = \"release view $RELEASED\" ] || exit 1\n" +
		"[ \"$4 $5 $6 $7\" != \"--json body --jq .body\" ] || printf '%s\\n' \"$BODY\"\n"
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte(gh), 0o700); err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = work
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q", "--bare", filepath.Join(dir, "origin"))
	git("init", "-q")
	git("remote", "add", "origin", filepath.Join(dir, "origin"))
	git("commit", "-q", "--allow-empty", "-m", "one")
	git("tag", "-a", "v1.3.0", "-m", "x")
	git("tag", "v1.4.0")
	old := git("rev-parse", "HEAD")
	git("commit", "-q", "--allow-empty", "-m", "two")
	git("tag", "-a", "v1.5.0", "-m", "x")
	git("push", "-q", "origin", "v1.3.0", "v1.4.0", "v1.5.0")

	changelog := func(version, unreleased string) string {
		return "## Unreleased\n\n" + unreleased + "\n## " + version + "\n\n- New.\n\n## 1.2.0\n\n- First.\n"
	}
	script, err := filepath.Abs("scripts/plan-release.sh")
	if err != nil {
		t.Fatal(err)
	}
	const push, dispatch, afterCI = "push", "workflow_dispatch", "workflow_run"
	for _, tc := range []struct {
		name, event, ref, input, changelog, released string
		want                                         string // empty: refused
	}{
		{"after ci, new version", afterCI, "refs/heads/main", "", changelog("2.0.0", ""), "v1.5.0", "release=true\ntag=v2.0.0\nnotes=false\n"},
		{"after ci, released already", afterCI, "refs/heads/main", "", changelog("1.5.0", ""), "v1.5.0", "release=false\ntag=v1.5.0\nnotes=false\n"},
		{"after ci, released without a text", afterCI, "refs/heads/main", "", changelog("1.5.0", ""), "v1.5.0 empty", "release=false\ntag=v1.5.0\nnotes=true\n"},
		{"after ci, released, work in progress", afterCI, "refs/heads/main", "", changelog("1.5.0", "- Next.\n"), "v1.5.0", "release=false\ntag=v1.5.0\nnotes=false\n"},
		{"after ci, tagged on another commit, no release", afterCI, "refs/heads/main", "", changelog("1.4.0", ""), "", "release=false\ntag=v1.4.0\nnotes=false\n"},
		{"after ci, tagged here, no release", afterCI, "refs/heads/main", "", changelog("1.5.0", ""), "", "release=false\ntag=v1.5.0\nnotes=false\n"},
		{"after ci, release without a tag (draft)", afterCI, "refs/heads/main", "", changelog("2.0.0", ""), "v2.0.0", "release=false\ntag=v2.0.0\nnotes=false\n"},
		{"after ci, only Unreleased", afterCI, "refs/heads/main", "", "## Unreleased\n\n- Next.\n", "", "release=false\ntag=\nnotes=false\n"},
		{"after ci, new version but changes under Unreleased", afterCI, "refs/heads/main", "", changelog("2.0.0", "- Next.\n"), "", ""},
		{"after ci, new version with a date", afterCI, "refs/heads/main", "", changelog("2.0.0 - 2026-11-25", ""), "", ""},
		{"after ci, new version in brackets", afterCI, "refs/heads/main", "", changelog("[2.0.0]", ""), "", ""},
		{"after ci, heading that is no version", afterCI, "refs/heads/main", "", changelog("$(id)", ""), "", ""},
		{"after ci, Unreleased in brackets", afterCI, "refs/heads/main", "", strings.Replace(changelog("2.0.0", ""), "## Unreleased", "## [Unreleased]", 1), "", ""},
		{"push to main itself", push, "refs/heads/main", "", changelog("2.0.0", ""), "", ""},
		{"pull request", "pull_request", "refs/pull/1/merge", "", changelog("2.0.0", ""), "", ""},
		{"tag", push, "refs/tags/v1.5.0", "", changelog("1.5.0", ""), "", "release=true\ntag=v1.5.0\nnotes=false\n"},
		{"tag that is not the newest", push, "refs/tags/v1.5.0", "", changelog("2.0.0", ""), "", ""},
		{"pre-release tag", push, "refs/tags/v2.0.0-rc1", "", changelog("1.5.0", "- Next.\n"), "v1.5.0", "release=true\ntag=v2.0.0-rc1\nnotes=false\n"},
		{"by hand", dispatch, "refs/heads/main", "v2.0.0", changelog("2.0.0", ""), "", "release=true\ntag=v2.0.0\nnotes=false\n"},
		{"by hand, tagged here", dispatch, "refs/heads/main", "v1.5.0", changelog("1.5.0", ""), "", "release=true\ntag=v1.5.0\nnotes=false\n"},
		{"by hand, tagged on another commit", dispatch, "refs/heads/main", "v1.4.0", changelog("1.4.0", ""), "", ""},
		{"by hand, annotated on another commit", dispatch, "refs/heads/main", "v1.3.0", changelog("1.3.0", ""), "", ""},
		{"by hand, other branch", dispatch, "refs/heads/test", "v2.0.0", changelog("2.0.0", ""), "", ""},
		{"by hand, from a tag", dispatch, "refs/tags/v1.5.0", "v1.5.0", changelog("1.5.0", ""), "", ""},
		{"by hand, no tag", dispatch, "refs/heads/main", "", changelog("2.0.0", ""), "", ""},
		{"by hand, output injection", dispatch, "refs/heads/main", "v2.0.0\nrelease=true", changelog("2.0.0", ""), "", ""},
	} {
		file := filepath.Join(t.TempDir(), "CHANGELOG.md")
		if err := os.WriteFile(file, []byte(tc.changelog), 0o600); err != nil {
			t.Fatal(err)
		}
		refType := "branch"
		if strings.HasPrefix(tc.ref, "refs/tags/") {
			refType = "tag"
		}
		released, empty, _ := strings.Cut(tc.released, " ")
		body := "Text."
		if empty != "" {
			body = "\r\n \n"
		}
		cmd := exec.Command("sh", script, file)
		cmd.Dir = work
		cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "RELEASED="+released, "BODY="+body,
			"GITHUB_ACTIONS=true", "GITHUB_EVENT_NAME="+tc.event, "GITHUB_REF="+tc.ref, "GITHUB_REF_TYPE="+refType,
			"GITHUB_REF_NAME="+tc.ref[strings.LastIndex(tc.ref, "/")+1:], "GITHUB_SHA="+old, "INPUT_TAG="+tc.input, "DEFAULT_BRANCH=main")
		var stdout, stderr strings.Builder
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		err := cmd.Run()
		if (err == nil) != (tc.want != "") || stdout.String() != tc.want {
			t.Errorf("%s (old %s): %v\n%q\n%s", tc.name, old, err, stdout.String(), stderr.String())
		}
	}
}

// TestReleaseNotesReachGitHub: with changelog.disable GoReleaser also ignores
// --release-notes, and the release has no text.
func TestReleaseNotesReachGitHub(t *testing.T) {
	config, err := os.ReadFile(".goreleaser.yaml")
	if err != nil {
		t.Fatal(err)
	}
	_, changelog, ok := strings.Cut(string(config), "\nchangelog:")
	if ok && regexp.MustCompile(`(?m)^ +disable:`).MatchString(changelog) {
		t.Error(".goreleaser.yaml disables the changelog, so --release-notes is ignored")
	}
	workflow, err := os.ReadFile(".github/workflows/release.yml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(workflow), "--release-notes ") {
		t.Error("release.yml does not pass --release-notes")
	}
}

func less(a, b [3]int) bool {
	for i := range a {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return false
}
