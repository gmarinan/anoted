package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"anoted/internal/config"
	"anoted/internal/session"
)

func TestRunRetentionDryRunThenApply(t *testing.T) {
	root := t.TempDir()
	store := session.NewSQLiteStore(filepath.Join(t.TempDir(), "sessions.db"))
	if err := store.Open(); err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	now := time.Now()
	mk := func(name, transcript string, age time.Duration) string {
		dir := filepath.Join(root, name)
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "recording.wav"), []byte("wavdata"), 0o600); err != nil {
			t.Fatal(err)
		}
		if transcript != "" {
			if err := os.WriteFile(filepath.Join(dir, "transcript.txt"), []byte(transcript), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		ended := now.Add(-age)
		if _, err := store.Create(context.Background(), session.Record{
			Dir: dir, StartedAt: ended.Add(-time.Hour), EndedAt: ended, Status: session.StatusStopped,
		}); err != nil {
			t.Fatal(err)
		}
		return dir
	}
	old := mk("old", "notes", 40*24*time.Hour)
	untranscribed := mk("untranscribed", "", 40*24*time.Hour)
	fresh := mk("fresh", "notes", time.Hour)

	cfg := config.Default()
	cfg.OutputDir = root
	cfg.Retention.KeepDays = 30

	var out bytes.Buffer
	if err := runRetention(context.Background(), &out, cfg, store, now, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "--apply") || !strings.Contains(out.String(), filepath.Join(old, "recording.wav")) {
		t.Fatalf("dry run output:\n%s", out.String())
	}
	for _, d := range []string{old, untranscribed, fresh} {
		if _, err := os.Stat(filepath.Join(d, "recording.wav")); err != nil {
			t.Fatalf("dry run deleted %s", d)
		}
	}

	out.Reset()
	if err := runRetention(context.Background(), &out, cfg, store, now, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(old, "recording.wav")); !os.IsNotExist(err) {
		t.Fatal("old transcribed audio should be gone")
	}
	if _, err := os.Stat(filepath.Join(old, "transcript.txt")); err != nil {
		t.Fatal("transcript must stay")
	}
	for _, d := range []string{untranscribed, fresh} {
		if _, err := os.Stat(filepath.Join(d, "recording.wav")); err != nil {
			t.Fatalf("%s must keep its audio", d)
		}
	}
	if !strings.Contains(out.String(), "Keeping 1 untranscribed") {
		t.Fatalf("apply output:\n%s", out.String())
	}
}

func TestRunRetentionWithoutLimitsDeletesNothing(t *testing.T) {
	root := t.TempDir()
	store := session.NewSQLiteStore(filepath.Join(t.TempDir(), "sessions.db"))
	if err := store.Open(); err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	cfg := config.Default()
	cfg.OutputDir = root

	var out bytes.Buffer
	if err := runRetention(context.Background(), &out, cfg, store, time.Now(), true); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "nothing to delete") {
		t.Fatalf("output:\n%s", out.String())
	}
}
