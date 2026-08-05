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
	args := pipInstallArgs("/usr/bin/python", "-U", "torch", "--index-url", "https://example.test")
	if args[0] != "/usr/bin/python" || args[1] != "-m" || args[2] != "pip" || args[3] != "install" {
		t.Fatalf("unexpected prefix: %v", args[:4])
	}
	// pip parses a second "install" token as a package name, so the subcommand
	// must appear exactly once no matter what the caller passes.
	installs := 0
	foundBar, foundVerbose := false, false
	for i, a := range args {
		if a == "install" {
			installs++
		}
		if a == "--progress-bar" && i+1 < len(args) && args[i+1] == "on" {
			foundBar = true
		}
		if a == "-v" {
			foundVerbose = true
		}
	}
	if installs != 1 {
		t.Fatalf("expected exactly one \"install\" token, got %d: %v", installs, args)
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

func TestHasCmdDirection(t *testing.T) {
	// hasCmd once returned the inverse of LookPath, which made the Windows
	// winget gate abort on exactly the machines where winget exists.
	if !hasCmd("go") {
		t.Fatal("hasCmd(go) = false, want true (go must be on PATH to run this test)")
	}
	if hasCmd("anoted-definitely-not-a-real-binary") {
		t.Fatal("hasCmd(nonexistent) = true, want false")
	}
}
