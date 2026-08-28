//go:build windows

package session

import "os"

// processAlive reports whether pid names a live process.
//
// os.Process.Signal rejects signal 0 on Windows, so the Unix trick does not
// port. FindProcess is enough here: unlike Unix it calls OpenProcess and fails
// when the pid does not exist.
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	if pid == os.Getpid() {
		// Our own pid in the file means this process already holds the lock, so
		// it is very much alive. Treating it as stale let a second acquire in
		// the same process quietly steal it.
		return true
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	_ = proc.Release()
	return true
}
