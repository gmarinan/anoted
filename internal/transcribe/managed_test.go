package transcribe

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestManagedVenvDir(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/tmp/anoted-test-data")
	got := ManagedVenvDir()
	want := filepath.Join("/tmp/anoted-test-data", "anoted", "whisper-venv")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestManagedWhisperBinary(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/tmp/anoted-test-data")
	got := ManagedWhisperBinary()
	if !filepath.IsAbs(got) {
		t.Fatalf("expected absolute path, got %q", got)
	}
	if filepath.Base(got) != "whisper" {
		t.Fatalf("expected whisper binary, got %q", got)
	}
}

func TestIsManagedBinary(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/tmp/anoted-test-data")
	if IsManagedBinary("/usr/bin/whisper") {
		t.Fatal("system whisper should not be managed")
	}
	if !IsManagedBinary(ManagedWhisperBinary()) {
		t.Fatal("managed path should match")
	}
}

func TestInstallHint(t *testing.T) {
	hint := InstallHint()
	if hint == "" {
		t.Fatal("empty hint")
	}
	if !strings.Contains(hint, "anoted") && !strings.Contains(hint, "whisper-venv") {
		t.Fatalf("unexpected hint: %s", hint)
	}
}

func TestFirstPythonAbsolute(t *testing.T) {
	t.Setenv("PATH", "/bin")
	py := firstPython()
	if py == "" {
		t.Skip("no python at standard paths")
	}
	if !filepath.IsAbs(py) && py != "python3" && py != "python" {
		t.Fatalf("unexpected python path: %q", py)
	}
}
