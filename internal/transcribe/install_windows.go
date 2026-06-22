//go:build windows

package transcribe

import "fmt"

// InstallCommand is not used for a single shell command; prefer InstallManaged.
func InstallCommand() ([]string, string, bool) {
	return nil, "", false
}

// InstallHint returns a human-readable install suggestion.
func InstallHint() string {
	return fmt.Sprintf("anoted setup  # or managed venv: %s", ManagedWhisperBinary())
}

// PacmanHint documents optional system packages (unused on Windows).
func PacmanHint() string {
	return ""
}

// WhisperCppHint documents the optional whisper.cpp path.
func WhisperCppHint() string {
	return "build whisper.cpp from https://github.com/ggml-org/whisper.cpp"
}

// PythonInstallHint returns how to install Python on Windows.
func PythonInstallHint() string {
	return "install: winget install Python.Python.3.12"
}
