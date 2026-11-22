package secmem

import "syscall"

func lock(b []byte) error   { return syscall.Mlock(b) }
func unlock(b []byte) error { return syscall.Munlock(b) }

const (
	prSetDumpable = 4
	prGetDumpable = 3
)

// ProtectProcess keeps secrets out of core dumps: the process is marked not
// dumpable (prctl PR_SET_DUMPABLE 0), which also stops other processes of
// the same user from attaching with ptrace.
func ProtectProcess() error {
	if _, _, errno := syscall.RawSyscall(syscall.SYS_PRCTL, prSetDumpable, 0, 0); errno != 0 {
		return errno
	}
	return nil
}

// dumpable reports the dumpable flag (for tests).
func dumpable() (bool, error) {
	r, _, errno := syscall.RawSyscall(syscall.SYS_PRCTL, prGetDumpable, 0, 0)
	if errno != 0 {
		return false, errno
	}
	return r != 0, nil
}
