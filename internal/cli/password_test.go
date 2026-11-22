package cli

import (
	"strings"
	"testing"

	"github.com/macsnoeren/backup-usb/internal/password"
)

func TestPasswordSource(t *testing.T) {
	c := &ctx{env: &Env{Stdin: strings.NewReader("from-the-pipe\n")}}
	b, err := c.passwordSource(true).Password("x")
	if err != nil || string(b.Bytes()) != "from-the-pipe" {
		t.Fatal(err)
	}
	if _, ok := c.passwordSource(false).(password.Console); !ok {
		t.Fatal("without --password-stdin the console is used")
	}
}
