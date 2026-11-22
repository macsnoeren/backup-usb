package password

import (
	"fmt"
	"io"

	"github.com/macsnoeren/backup-usb/internal/secmem"
)

// Console asks on the terminal without echoing the input. Prompts go to
// Out (usually stderr).
type Console struct {
	Out io.Writer
}

// Password implements Source.
func (c Console) Password(prompt string) (*secmem.Buffer, error) {
	tty, restore, err := openTTYNoEcho()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tty.Close() }()
	fmt.Fprint(c.Out, prompt)
	buf, err := ReadLine(tty)
	restore()
	fmt.Fprintln(c.Out)
	return buf, err
}
