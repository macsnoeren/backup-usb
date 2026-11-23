package archive

import "io/fs"

// ownerOf is not used on Windows: only the modification time is kept.
func ownerOf(fs.FileInfo) (uid, gid uint32, ok bool) { return 0, 0, false }
