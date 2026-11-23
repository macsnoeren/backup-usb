package cli

import (
	"github.com/macsnoeren/backup-usb/internal/password"
)

// passwordSource returns where passwords come from: stdin with
// --password-stdin (also the pipe from the GUI), otherwise the console
// without echo. Never from arguments or environment variables.
func (c *ctx) passwordSource(fromStdin bool) password.Source {
	// With --json-stream the GUI sends the password as the first line on
	// stdin.
	if fromStdin || c.stream != nil {
		return password.Reader{R: c.env.Stdin}
	}
	if c.env.Console != nil {
		return c.env.Console
	}
	return password.Console{Out: c.env.Stderr}
}
