//go:build !windows

package session

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// processAlive reports whether pid names a live process.
//
// Signal 0 runs the existence and permission checks without delivering
// anything, which is the standard way to ask this on Unix.
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
	err = proc.Signal(syscall.Signal(0))
	if err == nil {
		return true
	}
	// EPERM means the process exists but belongs to another user.
	return errors.Is(err, os.ErrPermission)
}

// processIsAnoted reports whether pid names an anoted process, not just a
// live one. Signal 0 answers for any process with that number — including a
// kernel thread, and a root-owned one returns EPERM, which reads as alive —
// so after a reboot a stale pid file can collide with an unrelated process
// and lock every later launch out. The pid only counts when its command line
// is anoted's. On systems without /proc the Signal 0 answer stands.
func processIsAnoted(pid int) bool {
	if pid == os.Getpid() {
		// We hold the lock whatever the binary is called; see the matching
		// case in processAlive guarding a second acquire in this process.
		return true
	}
	if _, err := os.Stat("/proc/self/cmdline"); err != nil {
		return true
	}
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if os.IsNotExist(err) {
		return false // it died between the liveness probe and this read
	}
	if err != nil {
		return true // unreadable: keep the Signal 0 answer
	}
	return isAnotedCmdline(data)
}

// isAnotedCmdline reports whether a NUL-separated /proc cmdline belongs to
// an anoted process, by its executable's base name.
func isAnotedCmdline(data []byte) bool {
	first := data
	if i := bytes.IndexByte(data, 0); i >= 0 {
		first = data[:i]
	}
	// Contains, not equality, so dev builds (anoted-fix, ./bin/anoted) are
	// still recognized; the miss direction only ever says "stale".
	return strings.Contains(filepath.Base(string(first)), "anoted")
}
