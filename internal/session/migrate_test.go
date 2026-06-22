package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMigrateLegacyIfEmpty(t *testing.T) {
	dir := t.TempDir()
	legacyDir := filepath.Join(dir, "meetctl")
	currentDir := filepath.Join(dir, "anoted")
	if err := os.MkdirAll(legacyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(currentDir, 0o755); err != nil {
		t.Fatal(err)
	}

	legacyDB := filepath.Join(legacyDir, "sessions.db")
	legacyStore := NewSQLiteStore(legacyDB)
	if err := legacyStore.Open(); err != nil {
		t.Fatal(err)
	}
	id, err := legacyStore.Create(Record{
		Dir:       "/tmp/session",
		Provider:  ProviderGoogleMeet,
		Platform:  "linux",
		Backend:   "pipewire",
		StartedAt: mustTime("2026-06-19T13:08:14Z"),
		Status:    StatusStopped,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := legacyStore.Close(); err != nil {
		t.Fatal(err)
	}

	currentDB := filepath.Join(currentDir, "sessions.db")
	t.Setenv("XDG_CONFIG_HOME", dir)
	// LegacyConfigDir uses UserConfigDir which respects XDG_CONFIG_HOME on Linux
	if err := MigrateLegacyIfEmpty(currentDB); err != nil {
		t.Fatal(err)
	}

	store := NewSQLiteStore(currentDB)
	if err := store.Open(); err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	recs, err := store.List(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 || recs[0].ID != id {
		t.Fatalf("got %+v want id %d", recs, id)
	}
}

func mustTime(s string) (t time.Time) {
	t, _ = time.Parse(time.RFC3339, s)
	return t
}
