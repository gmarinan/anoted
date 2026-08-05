package session

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"anoted/internal/config"
)

// MigrateLegacyIfEmpty copies the meetctl sessions database when the anoted store is empty.
func MigrateLegacyIfEmpty(currentDBPath string) error {
	legacyDir, err := config.LegacyConfigDir()
	if err != nil {
		return err
	}
	legacyDBPath := filepath.Join(legacyDir, "sessions.db")
	if _, err := os.Stat(legacyDBPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	count, err := sessionCount(currentDBPath)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	legacyCount, err := sessionCount(legacyDBPath)
	if err != nil {
		return err
	}
	if legacyCount == 0 {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(currentDBPath), 0o755); err != nil {
		return fmt.Errorf("create sessions dir: %w", err)
	}
	return copyFile(legacyDBPath, currentDBPath)
}

func sessionCount(dbPath string) (int, error) {
	if _, err := os.Stat(dbPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}
	store := NewSQLiteStore(dbPath)
	if err := store.Open(); err != nil {
		return 0, err
	}
	defer store.Close()
	recs, err := store.List(1)
	if err != nil {
		return 0, err
	}
	if len(recs) == 0 {
		return 0, nil
	}
	recs, err = store.List(10000)
	if err != nil {
		return 0, err
	}
	return len(recs), nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open legacy sessions db: %w", err)
	}
	defer in.Close()

	// Write to a temporary file and rename into place. Writing directly onto
	// sessions.db meant an interrupted copy left a truncated database, after
	// which every anoted subcommand failed with "database disk image is
	// malformed" and no indication of which file to remove.
	tmp := dst + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("create sessions db: %w", err)
	}
	defer func() {
		_ = out.Close()
		_ = os.Remove(tmp)
	}()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copy sessions db: %w", err)
	}
	if err := out.Sync(); err != nil {
		return fmt.Errorf("sync sessions db: %w", err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("close sessions db: %w", err)
	}
	if err := os.Rename(tmp, dst); err != nil {
		return fmt.Errorf("install sessions db: %w", err)
	}
	return nil
}
