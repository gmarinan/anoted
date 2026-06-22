//go:build !linux && !windows

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

// PacmanHint documents optional system packages (unused on non-Linux).
func PacmanHint() string {
	return ""
}

// WhisperCppHint documents the optional whisper.cpp path.
func WhisperCppHint() string {
	return "build whisper.cpp from https://github.com/ggml-org/whisper.cpp"
}

// PythonInstallHint returns how to install Python on this system.
func PythonInstallHint() string {
	return "install Python 3.8+ with venv support"
}
