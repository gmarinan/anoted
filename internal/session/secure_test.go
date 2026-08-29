package session

import (
	"os"
	"path/filepath"
	"testing"
)

// Creating new files with 0600 does nothing for the ones already on disk:
// O_CREATE and MkdirAll leave an existing path's mode alone, so an upgrade left
// the user's whole meeting history world-readable.
func TestSecureExistingTightensAnOldRecording(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "2026-06-23_12-35-42_google_meet")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	wav := filepath.Join(dir, "recording.wav")
	if err := os.WriteFile(wav, []byte("audio"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := SecureExisting(dir); err != nil {
		t.Fatalf("SecureExisting: %v", err)
	}

	for _, p := range []string{dir, wav} {
		info, err := os.Stat(p)
		if err != nil {
			t.Fatalf("stat %s: %v", p, err)
		}
		if perm := info.Mode().Perm(); perm&0o077 != 0 {
			t.Fatalf("%s still has mode %04o", p, perm)
		}
	}
}

func TestSecureFileLeavesAlreadyPrivateFilesAlone(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.db")
	if err := os.WriteFile(path, []byte("db"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := SecureFile(path); err != nil {
		t.Fatalf("SecureFile: %v", err)
	}
	info, _ := os.Stat(path)
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("mode = %04o, want 0600", perm)
	}
}

func TestSecureFileIgnoresMissingPaths(t *testing.T) {
	if err := SecureFile(filepath.Join(t.TempDir(), "nope")); err != nil {
		t.Fatalf("a missing file is not an error: %v", err)
	}
}
