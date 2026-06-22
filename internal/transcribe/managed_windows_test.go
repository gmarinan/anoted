//go:build windows

package transcribe

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestManagedVenvPathsWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-only test")
	}
	venv := `C:\Users\me\AppData\Local\anoted\whisper-venv`
	if got := venvWhisperPath(venv); got != filepath.Join(venv, "Scripts", "whisper.exe") {
		t.Fatalf("whisper path: %s", got)
	}
	if got := venvPythonPath(venv); got != filepath.Join(venv, "Scripts", "python.exe") {
		t.Fatalf("python path: %s", got)
	}
}
