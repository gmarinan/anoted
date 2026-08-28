package session

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newTestStore(t *testing.T) Store {
	t.Helper()
	store := NewSQLiteStore(filepath.Join(t.TempDir(), "sessions.db"))
	if err := store.Open(); err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func writeRecordingDir(t *testing.T, root, name string, meta Metadata) string {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	body, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("marshal metadata: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "metadata.json"), body, 0o600); err != nil {
		t.Fatalf("write metadata: %v", err)
	}
	return dir
}

// Killing anoted mid-recording left the row "active" with no end time, and
// nothing in the codebase ever read that status again.
func TestReconcileClosesRowsLeftActiveByACrash(t *testing.T) {
	root := t.TempDir()
	store := newTestStore(t)
	started := time.Now().Add(-time.Hour)

	dir := writeRecordingDir(t, root, "crashed", Metadata{StartedAt: started})
	id, err := store.Create(context.Background(), Record{Dir: dir, StartedAt: started, Status: StatusActive})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	res, err := Reconcile(context.Background(), store, root)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if res.Closed != 1 {
		t.Fatalf("Closed = %d, want 1", res.Closed)
	}

	got, err := store.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status == StatusActive {
		t.Fatal("row is still active after reconciliation")
	}
	// No end time was ever recorded, so the session must not claim a duration.
	if got.Status != StatusError {
		t.Fatalf("status = %q, want error for a recording that never finished", got.Status)
	}
}

// If the recorder finished and wrote ended_at but the database update was lost,
// the session is complete and should be marked stopped with its real duration.
func TestReconcileUsesMetadataEndTimeWhenPresent(t *testing.T) {
	root := t.TempDir()
	store := newTestStore(t)
	started := time.Now().Add(-90 * time.Minute).Round(time.Second)
	ended := started.Add(45 * time.Minute)

	dir := writeRecordingDir(t, root, "finished", Metadata{StartedAt: started, EndedAt: ended})
	id, err := store.Create(context.Background(), Record{Dir: dir, StartedAt: started, Status: StatusActive})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if _, err := Reconcile(context.Background(), store, root); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	got, err := store.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != StatusStopped {
		t.Fatalf("status = %q, want stopped", got.Status)
	}
	if got.Metadata.Duration != "45m0s" {
		t.Fatalf("duration = %q, want 45m0s", got.Metadata.Duration)
	}
}

// A recording whose insert failed, or one copied from another machine, was
// invisible in the UI forever: it could not be opened, transcribed or deleted.
func TestReconcileAdoptsRecordingsWithNoRow(t *testing.T) {
	root := t.TempDir()
	store := newTestStore(t)
	started := time.Now().Add(-2 * time.Hour)

	writeRecordingDir(t, root, "orphan", Metadata{
		StartedAt: started,
		EndedAt:   started.Add(time.Hour),
		Provider:  ProviderUnknown,
		Backend:   "pipewire",
	})
	// A directory that is not a recording must be ignored, not adopted.
	if err := os.MkdirAll(filepath.Join(root, "not-a-session"), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	res, err := Reconcile(context.Background(), store, root)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if res.Adopted != 1 {
		t.Fatalf("Adopted = %d, want 1", res.Adopted)
	}

	recs, err := store.List(context.Background(), 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(recs) != 1 || recs[0].Backend != "pipewire" {
		t.Fatalf("unexpected rows after adoption: %+v", recs)
	}

	// Reconciling again must not create duplicates.
	res, err = Reconcile(context.Background(), store, root)
	if err != nil {
		t.Fatalf("second Reconcile: %v", err)
	}
	if res.Adopted != 0 {
		t.Fatalf("second run adopted %d rows, want 0", res.Adopted)
	}
}

func TestReconcileToleratesMissingOutputDir(t *testing.T) {
	store := newTestStore(t)
	if _, err := Reconcile(context.Background(), store, filepath.Join(t.TempDir(), "does-not-exist")); err != nil {
		t.Fatalf("Reconcile should tolerate a missing output dir: %v", err)
	}
}
