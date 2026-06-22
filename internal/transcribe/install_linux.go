//go:build linux

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

// PacmanHint documents the optional system-wide Arch package (heavy deps).
func PacmanHint() string {
	return "sudo pacman -Syu && sudo pacman -S python-openai-whisper  # optional; pulls PyTorch/MKL via pacman"
}

// WhisperCppHint documents the optional whisper.cpp path (AUR / manual build).
func WhisperCppHint() string {
	if hasCmd("pacman") {
		return "yay -S whisper.cpp  # AUR; best for GPU; needs ggml model"
	}
	return "build whisper.cpp from https://github.com/ggml-org/whisper.cpp"
}

// PythonInstallHint returns how to install Python on Linux.
func PythonInstallHint() string {
	if hasCmd("pacman") {
		return "install: sudo pacman -S python"
	}
	if hasCmd("apt-get") {
		return "install: sudo apt install python3 python3-venv"
	}
	return "install Python 3.8+ with venv support"
}
