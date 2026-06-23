package transcribe

import (
	"io"
)

// EnsurePython finds a working Python interpreter or installs one when allowed.
func EnsurePython(out io.Writer, autoInstall bool) (string, error) {
	if py := discoverPython(); py != "" {
		return py, nil
	}
	return ensurePythonInstall(out, autoInstall)
}

// EnsureWhisper ensures Python is available and installs the managed Whisper venv.
func EnsureWhisper(out io.Writer, autoInstallPython bool) error {
	if _, err := EnsurePython(out, autoInstallPython); err != nil {
		return err
	}
	return InstallManaged(out)
}

// EnsureWhisperCaptured installs Whisper routing all subprocess output to out.
func EnsureWhisperCaptured(out io.Writer, autoInstallPython bool) error {
	if _, err := EnsurePython(out, autoInstallPython); err != nil {
		return err
	}
	return installManaged(out, out, out)
}
