//go:build !linux && !windows

package password

import "os"

func openTTYNoEcho() (*os.File, func(), error) { return nil, nil, ErrNoTTY }
