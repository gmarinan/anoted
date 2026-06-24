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

func TestPipInstallArgsIncludeProgress(t *testing.T) {
	args := pipInstallArgs("/usr/bin/python", "install", "-U", "torch", "--index-url", "https://example.test")
	if args[0] != "/usr/bin/python" || args[1] != "-m" || args[2] != "pip" || args[3] != "install" {
		t.Fatalf("unexpected prefix: %v", args[:4])
	}
	foundBar, foundVerbose := false, false
	for i, a := range args {
		if a == "--progress-bar" && i+1 < len(args) && args[i+1] == "on" {
			foundBar = true
		}
		if a == "-v" {
			foundVerbose = true
		}
	}
	if !foundBar {
		t.Fatal("expected --progress-bar on in pip args")
	}
	if !foundVerbose {
		t.Fatal("expected -v in pip args")
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
	py := discoverPython()
	if py == "" {
		t.Skip("no python at standard paths")
	}
	if !filepath.IsAbs(py) && py != "python3" && py != "python" {
		t.Fatalf("unexpected python path: %q", py)
	}
}
