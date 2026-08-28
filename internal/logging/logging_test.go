package logging

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetupFileReturnsLogger(t *testing.T) {
	logger, closer, err := SetupFile(slog.LevelInfo)
	if err != nil {
		t.Fatal(err)
	}
	defer closer.Close()
	if logger == nil {
		t.Fatal("expected logger")
	}
}

func TestSetupReturnsLogger(t *testing.T) {
	logger, closer, err := Setup(slog.LevelInfo)
	if err != nil {
		t.Fatal(err)
	}
	defer closer.Close()
	if logger == nil {
		t.Fatal("expected logger")
	}
}

func TestParseLevel(t *testing.T) {
	cases := map[string]slog.Level{
		"":      slog.LevelInfo,
		"info":  slog.LevelInfo,
		"debug": slog.LevelDebug,
		"warn":  slog.LevelWarn,
		"error": slog.LevelError,
	}
	for in, want := range cases {
		got, err := ParseLevel(in)
		if err != nil {
			t.Fatalf("ParseLevel(%q): %v", in, err)
		}
		if got != want {
			t.Fatalf("ParseLevel(%q) = %v, want %v", in, got, want)
		}
	}
	if _, err := ParseLevel("verbose"); err == nil {
		t.Fatal("an unknown level must be rejected, not silently treated as info")
	}
}

// The log had no size bound while `anoted watch` is designed to run from login
// for weeks.
func TestRotateIfLargeKeepsOneGeneration(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "anoted.log")

	if err := os.WriteFile(path, make([]byte, MaxLogBytes+1), 0o600); err != nil {
		t.Fatalf("seed log: %v", err)
	}
	if err := rotateIfLarge(path); err != nil {
		t.Fatalf("rotateIfLarge: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("oversized log should have been moved aside")
	}
	if _, err := os.Stat(path + ".1"); err != nil {
		t.Fatalf("previous generation missing: %v", err)
	}

	// A second rotation must replace the old generation, not accumulate.
	if err := os.WriteFile(path, make([]byte, MaxLogBytes+1), 0o600); err != nil {
		t.Fatalf("seed log again: %v", err)
	}
	if err := rotateIfLarge(path); err != nil {
		t.Fatalf("second rotateIfLarge: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	rotated := 0
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "anoted.log.") {
			rotated++
		}
	}
	if rotated != 1 {
		t.Fatalf("found %d rotated logs, want exactly 1", rotated)
	}
}

func TestRotateIfLargeLeavesSmallLogsAlone(t *testing.T) {
	path := filepath.Join(t.TempDir(), "anoted.log")
	if err := os.WriteFile(path, []byte("small"), 0o600); err != nil {
		t.Fatalf("seed log: %v", err)
	}
	if err := rotateIfLarge(path); err != nil {
		t.Fatalf("rotateIfLarge: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("small log should be untouched: %v", err)
	}
}
