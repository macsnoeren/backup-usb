//go:build !windows

package fsx

import "syscall"

// oNonblock makes opening a FIFO return at once instead of waiting for a
// writer.
const oNonblock = syscall.O_NONBLOCK
