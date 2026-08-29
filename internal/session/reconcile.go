package session

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

// ReconcileResult summarises what a startup reconciliation changed.
type ReconcileResult struct {
	Closed  int // rows left "active" by a crash, now closed
	Adopted int // recordings on disk that had no row at all
	Secured int // recordings whose permissions were tightened
}

// Reconcile makes the database agree with what is actually on disk.
//
// Two ways sessions used to disappear, both permanent and both silent:
//
//   - A row is inserted as "active" when recording starts and closed when it
//     stops. Nothing closed it if anoted died in between — SIGTERM at logout, a
//     crash, a power cut — and nothing ever read StatusActive afterwards, so the
//     row stayed active with no end time or duration forever.
//   - The Sessions list is built purely from the database. A recording whose
//     insert failed, or one copied in from another machine, existed on disk but
//     could not be opened, transcribed or deleted from the UI.
//
// Both are recoverable because every recording directory carries a
// metadata.json written at start. Reconcile closes stale rows using the
// recording's own timestamps and adopts orphaned directories.
func Reconcile(ctx context.Context, store Store, outputDir string) (ReconcileResult, error) {
	var res ReconcileResult
	if store == nil {
		return res, nil
	}

	recs, err := store.List(ctx, reconcileScanLimit)
	if err != nil {
		return res, fmt.Errorf("list sessions: %w", err)
	}

	known := make(map[string]bool, len(recs))
	for _, rec := range recs {
		if rec.Dir != "" {
			known[filepath.Clean(rec.Dir)] = true
		}
		if rec.Status != StatusActive {
			continue
		}
		if err := closeStaleRow(ctx, store, rec); err != nil {
			slog.Warn("could not close stale session row", "session_id", rec.ID, "dir", rec.Dir, "err", err)
			continue
		}
		res.Closed++
	}

	res.Secured = secureExistingRecordings(outputDir)

	adopted, err := adoptOrphans(ctx, store, outputDir, known)
	if err != nil {
		// Adoption is best-effort: a missing or unreadable output directory must
		// not stop anoted from starting.
		slog.Warn("could not scan for orphaned recordings", "dir", outputDir, "err", err)
		return res, nil
	}
	res.Adopted = adopted
	return res, nil
}

// reconcileScanLimit bounds the startup scan. Well above any realistic session
// count, but not unbounded.
const reconcileScanLimit = 10000

func closeStaleRow(ctx context.Context, store Store, rec Record) error {
	// Prefer the recording's own metadata: it is written at start and updated at
	// stop, so it knows more than the row does.
	meta, err := ReadMetadataFile(rec.Dir)
	switch {
	case err == nil && !meta.EndedAt.IsZero():
		// The recorder finished and wrote the end time; only the database update
		// was lost.
		rec.EndedAt = meta.EndedAt
		rec.Status = StatusStopped
	default:
		// No end time anywhere: the process died mid-recording. Mark it as such
		// rather than inventing a duration.
		rec.EndedAt = time.Time{}
		rec.Status = StatusError
	}
	if err == nil {
		rec.Metadata = meta
	}
	if !rec.EndedAt.IsZero() && !rec.StartedAt.IsZero() {
		rec.Metadata.EndedAt = rec.EndedAt
		rec.Metadata.Duration = rec.EndedAt.Sub(rec.StartedAt).Round(time.Second).String()
	}
	if err := store.Update(ctx, rec); err != nil {
		return fmt.Errorf("update session %d: %w", rec.ID, err)
	}
	slog.Info("closed stale session row", "session_id", rec.ID, "dir", rec.Dir, "status", rec.Status)
	return nil
}

func adoptOrphans(ctx context.Context, store Store, outputDir string, known map[string]bool) (int, error) {
	if outputDir == "" {
		return 0, nil
	}
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("read output dir %s: %w", outputDir, err)
	}

	adopted := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(outputDir, e.Name())
		if known[filepath.Clean(dir)] {
			continue
		}
		meta, err := ReadMetadataFile(dir)
		if err != nil {
			// Not a recording directory, or unreadable. Leave it alone.
			continue
		}
		rec := Record{
			Dir:       dir,
			Provider:  meta.Provider,
			Platform:  meta.Platform,
			Backend:   meta.Backend,
			StartedAt: meta.StartedAt,
			EndedAt:   meta.EndedAt,
			Status:    StatusStopped,
			Metadata:  meta,
		}
		if meta.EndedAt.IsZero() {
			rec.Status = StatusError
		}
		if _, err := store.Create(ctx, rec); err != nil {
			slog.Warn("could not adopt orphaned recording", "dir", dir, "err", err)
			continue
		}
		adopted++
		slog.Info("adopted orphaned recording", "dir", dir, "status", rec.Status)
	}
	return adopted, nil
}

// secureExistingRecordings restricts recordings written before anoted started
// creating them owner-only.
//
// Creating new files with 0600 does nothing for the ones already on disk:
// O_CREATE and MkdirAll both leave an existing path's mode alone. Without this
// an upgrade leaves the user's entire meeting history world-readable, which is
// the exact thing the permission change was meant to prevent.
func secureExistingRecordings(outputDir string) int {
	if outputDir == "" {
		return 0
	}
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return 0
	}
	secured := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(outputDir, e.Name())
		info, err := e.Info()
		if err != nil || info.Mode().Perm()&0o077 == 0 {
			continue
		}
		if err := SecureExisting(dir); err != nil {
			slog.Warn("could not restrict recording permissions", "dir", dir, "err", err)
			continue
		}
		secured++
	}
	if secured > 0 {
		slog.Info("restricted recording permissions to owner-only", "count", secured)
	}
	return secured
}
