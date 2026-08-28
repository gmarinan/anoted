//go:build windows

package session

import "os"

// processAlive reports whether pid names a live process.
//
// os.Process.Signal rejects signal 0 on Windows, so the Unix trick does not
// port. FindProcess is enough here: unlike Unix it calls OpenProcess and fails
// when the pid does not exist.
func processAlive(pid int) bool {
	if pid <= 0 || pid == os.Getpid() {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	_ = proc.Release()
	return true
}
