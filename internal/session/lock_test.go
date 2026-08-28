package session

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestInstanceLockRejectsASecondHolder(t *testing.T) {
	dir := t.TempDir()

	first, err := AcquireInstanceLock(dir)
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}

	if _, err := AcquireInstanceLock(dir); !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("second acquire error = %v, want ErrAlreadyRunning", err)
	}

	if err := first.Release(); err != nil {
		t.Fatalf("release: %v", err)
	}
	second, err := AcquireInstanceLock(dir)
	if err != nil {
		t.Fatalf("acquire after release: %v", err)
	}
	_ = second.Release()
}

// A pid file left behind by a crash must not lock the user out of their own
// app forever.
func TestInstanceLockReclaimsAStalePidFile(t *testing.T) {
	dir := t.TempDir()
	// PID 0 is never a live process, so this stands in for a crashed run.
	if err := os.WriteFile(filepath.Join(dir, "anoted.pid"), []byte("0\n"), 0o600); err != nil {
		t.Fatalf("seed pid file: %v", err)
	}

	lock, err := AcquireInstanceLock(dir)
	if err != nil {
		t.Fatalf("a stale pid file must be reclaimed, got: %v", err)
	}
	_ = lock.Release()
}

func TestInstanceLockReclaimsAGarbagePidFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "anoted.pid"), []byte("not a pid"), 0o600); err != nil {
		t.Fatalf("seed pid file: %v", err)
	}
	lock, err := AcquireInstanceLock(dir)
	if err != nil {
		t.Fatalf("an unparseable pid file must be reclaimed, got: %v", err)
	}
	_ = lock.Release()
}

func TestInstanceLockPidFileIsOwnerOnly(t *testing.T) {
	dir := t.TempDir()
	lock, err := AcquireInstanceLock(dir)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	defer lock.Release()

	info, err := os.Stat(filepath.Join(dir, "anoted.pid"))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm&0o077 != 0 {
		t.Fatalf("pid file mode %04o is group/world accessible", perm)
	}
}

func TestReleaseIsSafeOnNil(t *testing.T) {
	var lock *InstanceLock
	if err := lock.Release(); err != nil {
		t.Fatalf("releasing a nil lock should be a no-op: %v", err)
	}
}
