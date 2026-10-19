//go:build windows

package transcribe

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
)

func TestFormatCmdError_Exit9009(t *testing.T) {
	// os.ProcessState cannot be faked, so produce a real exit 9009 — the code
	// cmd.exe reports when a command is not found.
	err := exec.Command("cmd", "/c", "exit 9009").Run()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 9009 {
		t.Fatalf("expected exit 9009, got %v", err)
	}
	got := formatCmdError("create venv", err)
	if got == nil {
		t.Fatal("expected error")
	}
	if msg := got.Error(); !strings.Contains(msg, "command not found (exit 9009)") {
		t.Fatalf("9009 should be explained as a missing command, got %q", msg)
	}
}
