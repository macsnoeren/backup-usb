//go:build !windows

package archive

import (
	"io/fs"
	"syscall"
)

// ownerOf returns the numeric owner and group of a file.
func ownerOf(info fs.FileInfo) (uid, gid uint32, ok bool) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, 0, false
	}
	return st.Uid, st.Gid, true
}
