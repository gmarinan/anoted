package transcribe

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestManagedVenvPathsUnix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix-only test")
	}
	venv := "/home/me/.local/share/anoted/whisper-venv"
	if got := venvWhisperPath(venv); got != filepath.Join(venv, "bin", "whisper") {
		t.Fatalf("whisper path: %s", got)
	}
	if got := venvPythonPath(venv); got != filepath.Join(venv, "bin", "python") {
		t.Fatalf("python path: %s", got)
	}
}

func TestManagedVenvDirForAppLinux(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix-only test")
	}
	got := managedVenvDirForApp("anoted")
	wantSuffix := filepath.Join(".local", "share", "anoted", managedVenvName)
	if !filepath.IsAbs(got) {
		t.Fatalf("expected absolute path, got %s", got)
	}
	if filepath.Base(filepath.Dir(filepath.Dir(got))) != "share" && !containsPath(got, wantSuffix) {
		// home-relative layout
		if filepath.Base(got) != managedVenvName {
			t.Fatalf("unexpected venv dir: %s", got)
		}
	}
}

func containsPath(path, suffix string) bool {
	return len(path) >= len(suffix) && path[len(path)-len(suffix):] == suffix
}
