package recorder

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Stopping and restarting a recording inside the same second produced the same
// second-resolution directory name twice. MkdirAll accepted it, and the second
// take then overwrote the first one's audio.
func TestCreateSessionDirNeverReusesADirectory(t *testing.T) {
	root := t.TempDir()
	cfg := SessionConfig{OutputRoot: root, Provider: "google_meet"}

	first, err := createSessionDir(cfg)
	if err != nil {
		t.Fatalf("first createSessionDir: %v", err)
	}
	marker := filepath.Join(first, "keep.txt")
	if err := os.WriteFile(marker, []byte("original take"), SessionFileMode); err != nil {
		t.Fatalf("write marker: %v", err)
	}

	second, err := createSessionDir(cfg)
	if err != nil {
		t.Fatalf("second createSessionDir: %v", err)
	}
	if second == first {
		t.Fatalf("second session reused %s", first)
	}
	if body, err := os.ReadFile(marker); err != nil || string(body) != "original take" {
		t.Fatalf("first recording was clobbered: body=%q err=%v", body, err)
	}
	if !strings.HasPrefix(filepath.Base(second), filepath.Base(first)) {
		t.Fatalf("collision suffix should extend the timestamp name: %s", second)
	}
}

// Recordings and the directories holding them must not be readable by other
// users on a shared machine.
func TestCreateSessionDirIsOwnerOnly(t *testing.T) {
	dir, err := createSessionDir(SessionConfig{OutputRoot: t.TempDir(), Provider: "teams"})
	if err != nil {
		t.Fatalf("createSessionDir: %v", err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm&0o077 != 0 {
		t.Fatalf("session dir mode %04o is group/world accessible", perm)
	}
}
