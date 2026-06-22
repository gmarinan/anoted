package transcribe

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveManagedVenvDirUsesLegacy(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_DATA_HOME", root)

	current := filepath.Join(root, "anoted", managedVenvName, "bin")
	legacy := filepath.Join(root, "meetctl", managedVenvName, "bin")
	if err := os.MkdirAll(legacy, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "whisper"), []byte("stub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(current, 0o755); err != nil {
		t.Fatal(err)
	}

	got := resolveManagedVenvDir()
	want := filepath.Join(root, "meetctl", managedVenvName)
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if !ManagedWhisperInstalled() {
		t.Fatal("expected legacy whisper to be detected")
	}
}
