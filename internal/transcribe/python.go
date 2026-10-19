package transcribe

import (
	"context"
	"os/exec"
	"strings"
)

// verifyPython runs a minimal check that the interpreter is a real Python 3.
//
//nolint:contextcheck // short probe, deliberately not tied to a caller's ctx
func verifyPython(path string) bool {
	if path == "" || isStorePythonStub(path) {
		return false
	}
	cmd := exec.CommandContext(context.Background(), path, "-c", "import sys; print(sys.version_info[0], sys.version_info[1])")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}
	s := strings.ToLower(string(out))
	if strings.Contains(s, "microsoft store") {
		return false
	}
	return strings.TrimSpace(s) != ""
}

// isStorePythonStub reports Windows App Execution Alias shims that are not real Python.
func isStorePythonStub(path string) bool {
	lower := strings.ToLower(strings.ReplaceAll(path, "/", `\`))
	if !strings.Contains(lower, `\windowsapps\`) {
		return false
	}
	base := strings.ToLower(strings.TrimPrefix(lower, strings.ToLower(lower[:len(lower)-len(`\python.exe`)])))
	_ = base
	return strings.HasSuffix(lower, `\python.exe`) || strings.HasSuffix(lower, `\python3.exe`)
}
