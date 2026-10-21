//go:build !windows

package auth

import (
	"os"

	"golang.org/x/sys/unix"
)

// tryLockFile takes a non-blocking exclusive advisory lock on f.
func tryLockFile(f *os.File) bool {
	return unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB) == nil
}

func unlockFile(f *os.File) {
	_ = unix.Flock(int(f.Fd()), unix.LOCK_UN)
}
