package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSQLiteStoreCreateList(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "sessions.db")
	store := NewSQLiteStore(dbPath)
	if err := store.Open(); err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	started := time.Now().UTC().Truncate(time.Second)
	rec := Record{
		Dir:       t.TempDir(),
		Provider:  ProviderTeams,
		Platform:  "linux",
		Backend:   "dummy",
		StartedAt: started,
		Status:    StatusActive,
		Metadata: Metadata{
			StartedAt: started,
			Provider:  ProviderTeams,
			Platform:  "linux",
			Backend:   "dummy",
			Manual:    true,
		},
	}

	id, err := store.Create(rec)
	if err != nil {
		t.Fatal(err)
	}
	if id == 0 {
		t.Fatal("expected non-zero id")
	}

	list, err := store.List(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 session, got %d", len(list))
	}
	if list[0].Provider != ProviderTeams {
		t.Fatalf("unexpected provider %s", list[0].Provider)
	}
}

func TestSQLiteStoreDelete(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "sessions.db")
	store := NewSQLiteStore(dbPath)
	if err := store.Open(); err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	dir := t.TempDir()
	started := time.Now().UTC().Truncate(time.Second)
	id, err := store.Create(Record{
		Dir:       dir,
		Provider:  ProviderTeams,
		Platform:  "linux",
		Backend:   "dummy",
		StartedAt: started,
		Status:    StatusStopped,
		Metadata: Metadata{
			StartedAt: started,
			Provider:  ProviderTeams,
			Platform:  "linux",
			Backend:   "dummy",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	rec, err := store.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	if err := Remove(store, rec); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("expected dir removed, stat err=%v", err)
	}
	list, err := store.List(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("expected 0 sessions, got %d", len(list))
	}
}

func TestWriteMetadataFile(t *testing.T) {
	dir := t.TempDir()
	meta := Metadata{
		StartedAt: time.Now(),
		Provider:  ProviderGoogleMeet,
		Platform:  "linux",
		Backend:   "dummy",
	}
	if err := WriteMetadataFile(dir, meta); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateMetadataEndedPreservesFields(t *testing.T) {
	dir := t.TempDir()
	started := time.Date(2026, 6, 23, 16, 30, 0, 0, time.UTC)
	meta := Metadata{
		StartedAt:    started,
		Provider:     ProviderGoogleMeet,
		Platform:     "linux",
		Backend:      "pipewire",
		SystemDevice: "monitor",
		Manual:       true,
	}
	if err := WriteMetadataFile(dir, meta); err != nil {
		t.Fatal(err)
	}
	ended := started.Add(3 * time.Minute)
	if err := UpdateMetadataEnded(dir, started, ended); err != nil {
		t.Fatal(err)
	}
	got, err := ReadMetadataFile(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Provider != ProviderGoogleMeet {
		t.Fatalf("provider %q", got.Provider)
	}
	if got.Duration != "3m0s" {
		t.Fatalf("duration %q", got.Duration)
	}
	if got.EndedAt.IsZero() {
		t.Fatal("expected ended_at")
	}
}
