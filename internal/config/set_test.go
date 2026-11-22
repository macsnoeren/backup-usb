package config

import "testing"

func TestSet(t *testing.T) {
	c := Defaults()
	cases := []struct{ key, value, want string }{
		{"keepPerDisk", "10", "10"},
		{"verifyAfterBackup", "false", "false"},
		{"folderPattern", `{year}-{week}`, `"{year}-{week}"`},
		{"schedule", "day", `"day"`},
		{"exclude", "*.bak, *.tmp", `["*.bak","*.tmp"]`},
		{"storeExtensions", `["jpg"]`, `["jpg"]`},
		{"sources", `[{"tag":"A","path":"/a"}]`, `[{"tag":"A","path":"/a"}]`},
	}
	for _, tc := range cases {
		_, got, err := c.Set(tc.key, tc.value)
		if err != nil || got != tc.want {
			t.Errorf("%s=%s: %q %v", tc.key, tc.value, got, err)
		}
	}
	for _, bad := range [][2]string{{"nope", "1"}, {"keepPerDisk", "x"}, {"ejectAfter", "maybe"}, {"sources", "a,b"}} {
		if _, _, err := c.Set(bad[0], bad[1]); err == nil {
			t.Errorf("%v accepted", bad)
		}
	}
	if len(Keys()) < 30 {
		t.Fatal("keys")
	}
}
