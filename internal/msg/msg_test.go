package msg

import (
	"strings"
	"testing"

	_ "github.com/macsnoeren/backup-usb/internal/testutil" // -update flag
)

func TestEveryKeyHasEnglishAndMatchingVerbs(t *testing.T) {
	for _, k := range Keys() {
		e := table[k]
		if e[EN] == "" {
			t.Errorf("%s: no English text", k)
		}
		if e[NL] != "" && strings.Count(e[NL], "%") != strings.Count(e[EN], "%") {
			t.Errorf("%s: format verbs differ between nl and en", k)
		}
	}
}

func TestFallback(t *testing.T) {
	if got := T(EN, "no.such.key"); got != "no.such.key" {
		t.Fatalf("got %q", got)
	}
	if got := T(EN, "version", "1", "x", "linux", "amd64"); !strings.Contains(got, "version 1") {
		t.Fatalf("got %q", got)
	}
}
