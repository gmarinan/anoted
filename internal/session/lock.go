package session

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ErrAlreadyRunning is returned when another anoted instance holds the lock.
var ErrAlreadyRunning = errors.New("another anoted instance is already running")

// InstanceLock is a PID file guarding a single anoted process per config dir.
type InstanceLock struct {
	path string
}

// AcquireInstanceLock claims exclusive use of the config directory.
//
// Nothing stopped two anoted processes from sharing one sessions.db, which is
// easy to hit by accident — a login autostart entry plus a manual launch. Both
// would record, both would write rows, and closing a recording used to find the
// other instance's row and leave its own active forever. That specific failure
// is fixed, but two processes recording the same meeting to different files is
// not something to allow silently.
func AcquireInstanceLock(dir string) (*InstanceLock, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create config dir %s: %w", dir, err)
	}
	path := filepath.Join(dir, "anoted.pid")

	for attempt := 0; attempt < 2; attempt++ {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			_, writeErr := fmt.Fprintf(f, "%d\n", os.Getpid())
			closeErr := f.Close()
			if writeErr != nil {
				_ = os.Remove(path)
				return nil, fmt.Errorf("write pid file: %w", writeErr)
			}
			if closeErr != nil {
				_ = os.Remove(path)
				return nil, fmt.Errorf("close pid file: %w", closeErr)
			}
			return &InstanceLock{path: path}, nil
		}
		if !os.IsExist(err) {
			return nil, fmt.Errorf("create pid file %s: %w", path, err)
		}

		// A pid file left by a crash must not lock the user out forever.
		// Being alive is not enough: a pid file surviving a reboot collides
		// with whatever number the new boot gave to another process — even a
		// kernel thread answers the liveness probe — and only a live anoted
		// process may hold the lock.
		pid, readErr := readPID(path)
		if readErr == nil && processAlive(pid) && processIsAnoted(pid) {
			return nil, fmt.Errorf("%w (pid %d)", ErrAlreadyRunning, pid)
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("remove stale pid file %s: %w", path, err)
		}
	}
	return nil, fmt.Errorf("could not acquire %s", path)
}

// Release removes the pid file. Safe to call on a nil lock.
func (l *InstanceLock) Release() error {
	if l == nil || l.path == "" {
		return nil
	}
	if err := os.Remove(l.path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove pid file %s: %w", l.path, err)
	}
	return nil
}

func readPID(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("read pid file %s: %w", path, err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("parse pid file %s: %w", path, err)
	}
	return pid, nil
}

// processAlive is implemented per platform: see lock_unix.go and
// lock_windows.go. os.Process.Signal is not usable on Windows, so the two
// cannot share one implementation.
