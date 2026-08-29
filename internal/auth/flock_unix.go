//go:build darwin || linux

package auth

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

func tryAdvisoryLock(file *os.File) error {
	err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
		return ErrRefreshInProgress
	}
	return err
}

func unlockAdvisory(file *os.File) error { return unix.Flock(int(file.Fd()), unix.LOCK_UN) }
