package session

import (
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
