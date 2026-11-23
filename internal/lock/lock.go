package lock

import "errors"

// ErrBusy means another instance holds the lock.
var ErrBusy = errors.New("another backup is running")

// Name is the lock name: the mutex Global\UsbBackup on Windows, the file
// /run/lock/usb-backup.lock on Linux.
const Name = "UsbBackup"
