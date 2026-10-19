package transcribe

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveManagedVenvDirUsesLegacy(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_DATA_HOME", root)
	t.Setenv("LOCALAPPDATA", root)

	current := venvBinDir(filepath.Join(root, "anoted", managedVenvName))
	legacyVenv := filepath.Join(root, "meetctl", managedVenvName)
	if err := os.MkdirAll(venvBinDir(legacyVenv), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(venvWhisperPath(legacyVenv), []byte("stub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(current, 0o755); err != nil {
		t.Fatal(err)
	}

	got := resolveManagedVenvDir()
	if got != legacyVenv {
		t.Fatalf("got %q want %q", got, legacyVenv)
	}
	if !ManagedWhisperInstalled() {
		t.Fatal("expected legacy whisper to be detected")
	}
}
