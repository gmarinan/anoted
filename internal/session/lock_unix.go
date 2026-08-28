//go:build !windows

package session

import (
	"errors"
	"os"
	"syscall"
)

// processAlive reports whether pid names a live process.
//
// Signal 0 runs the existence and permission checks without delivering
// anything, which is the standard way to ask this on Unix.
func processAlive(pid int) bool {
	if pid <= 0 || pid == os.Getpid() {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	if err == nil {
		return true
	}
	// EPERM means the process exists but belongs to another user.
	return errors.Is(err, os.ErrPermission)
}
