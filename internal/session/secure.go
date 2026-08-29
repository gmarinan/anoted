package session

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// tightenMode restricts an existing path to owner-only access.
//
// Creating files with 0600 is not enough on its own: os.OpenFile with O_CREATE
// and os.MkdirAll both leave the mode of an already-existing path alone, so
// every recording, database and log written before that change stayed
// world-readable. Anyone upgrading keeps their whole meeting history exposed
// unless it is corrected in place.
func tightenMode(path string, want fs.FileMode) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat %s: %w", path, err)
	}
	if info.Mode().Perm()&0o077 == 0 {
		return nil // already owner-only
	}
	if err := os.Chmod(path, want); err != nil {
		return fmt.Errorf("chmod %s: %w", path, err)
	}
	return nil
}

// SecureExisting restricts a recording directory and everything in it.
func SecureExisting(dir string) error {
	if dir == "" {
		return nil
	}
	if err := tightenMode(dir, SessionDirMode); err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read %s: %w", dir, err)
	}
	for _, e := range entries {
		mode := SessionFileMode
		if e.IsDir() {
			mode = SessionDirMode
		}
		if err := tightenMode(filepath.Join(dir, e.Name()), mode); err != nil {
			return err
		}
	}
	return nil
}

// SecureFile restricts a single file such as the database or the log.
func SecureFile(path string) error { return tightenMode(path, SessionFileMode) }

// Modes for privacy-sensitive artifacts, mirroring the recorder package so a
// session directory has one definition of "owner only".
const (
	SessionDirMode  fs.FileMode = 0o700
	SessionFileMode fs.FileMode = 0o600
)
