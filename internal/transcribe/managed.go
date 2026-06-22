package transcribe

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"anoted/internal/config"
)

const managedVenvName = "whisper-venv"

// ManagedVenvDir is the anoted-managed Python venv for OpenAI Whisper.
func ManagedVenvDir() string {
	return managedVenvDirForApp(config.AppName)
}

func legacyManagedVenvDir() string {
	return managedVenvDirForApp(config.LegacyAppName)
}

func resolveManagedVenvDir() string {
	current := ManagedVenvDir()
	if whisperInstalledIn(current) {
		return current
	}
	legacy := legacyManagedVenvDir()
	if whisperInstalledIn(legacy) {
		return legacy
	}
	return current
}

func whisperInstalledIn(venvDir string) bool {
	_, err := os.Stat(venvWhisperPath(venvDir))
	return err == nil
}

// ManagedWhisperBinary returns the whisper CLI path inside the managed venv.
func ManagedWhisperBinary() string {
	return venvWhisperPath(resolveManagedVenvDir())
}

// IsManagedBinary reports whether path is the anoted-managed whisper binary.
func IsManagedBinary(path string) bool {
	if path == "" {
		return false
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	for _, candidate := range []string{
		ManagedWhisperBinary(),
		venvWhisperPath(ManagedVenvDir()),
		venvWhisperPath(legacyManagedVenvDir()),
	} {
		absManaged, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}
		if absPath == absManaged {
			return true
		}
	}
	return false
}

// ManagedWhisperInstalled reports whether the managed venv whisper exists.
func ManagedWhisperInstalled() bool {
	_, err := os.Stat(ManagedWhisperBinary())
	return err == nil
}

// InstallManaged creates/updates the anoted venv with CPU PyTorch + openai-whisper.
// No root required; downloads from PyPI (~400–600 MB).
func InstallManaged(out io.Writer) error {
	py, err := findPython()
	if err != nil {
		return err
	}

	venv := resolveManagedVenvDir()
	if err := os.MkdirAll(filepath.Dir(venv), 0o755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	python := venvPythonPath(venv)
	if _, err := os.Stat(python); err != nil {
		fmt.Fprintf(out, "  Creating venv at %s\n", venv)
		if err := runCmd(py, "-m", "venv", venv); err != nil {
			return fmt.Errorf("create venv: %w", err)
		}
	}

	pip := venvPipPath(venv)
	steps := []struct {
		desc string
		args []string
	}{
		{"Upgrading pip", []string{pip, "install", "-U", "pip"}},
		{"Installing PyTorch (CPU)", []string{pip, "install", "torch", "--index-url", "https://download.pytorch.org/whl/cpu"}},
		{"Installing openai-whisper", []string{pip, "install", "-U", "openai-whisper"}},
	}
	for _, step := range steps {
		fmt.Fprintf(out, "  %s…\n", step.desc)
		if err := runCmd(step.args...); err != nil {
			return fmt.Errorf("%s: %w", step.desc, err)
		}
	}

	if !ManagedWhisperInstalled() {
		return fmt.Errorf("whisper binary missing after install")
	}
	fmt.Fprintf(out, "  ✓ Installed: %s\n", ManagedWhisperBinary())
	return nil
}

// UpgradeManagedTorchCUDA replaces CPU PyTorch with a CUDA build in the managed venv.
func UpgradeManagedTorchCUDA(out io.Writer) error {
	if !ManagedWhisperInstalled() {
		return fmt.Errorf("managed venv not installed — run anoted setup first")
	}
	pip := venvPipPath(resolveManagedVenvDir())
	indexes := []string{
		"https://download.pytorch.org/whl/cu126",
		"https://download.pytorch.org/whl/cu124",
		"https://download.pytorch.org/whl/cu121",
	}
	var lastErr error
	for _, idx := range indexes {
		fmt.Fprintf(out, "  Installing PyTorch (CUDA) from %s…\n", idx)
		err := runCmd(pip, "install", "-U", "torch", "--index-url", idx)
		if err != nil {
			lastErr = err
			continue
		}
		if ManagedTorchCUDAAvailable() {
			return nil
		}
		lastErr = fmt.Errorf("torch installed but CUDA not available")
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no CUDA torch wheel matched this system")
	}
	return lastErr
}

// ManagedTorchCUDAAvailable reports whether the managed venv can use CUDA.
func ManagedTorchCUDAAvailable() bool {
	python := venvPythonPath(resolveManagedVenvDir())
	cmd := exec.Command(python, "-c", "import torch; raise SystemExit(0 if torch.cuda.is_available() else 1)")
	return cmd.Run() == nil
}

func runCmd(args ...string) error {
	if len(args) == 0 {
		return fmt.Errorf("empty command")
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func findPython() (string, error) {
	if py := firstPython(); py != "" {
		return py, nil
	}
	return "", fmt.Errorf("python3 not found — %s", PythonInstallHint())
}

// FindPython returns the resolved Python interpreter path.
func FindPython() (string, error) {
	return findPython()
}

var pythonCandidates = []string{
	"python3",
	"python",
	"/usr/bin/python3",
	"/usr/bin/python",
	"/usr/local/bin/python3",
}

func firstPython() string {
	for _, name := range pythonCandidates {
		if path, ok := resolvePython(name); ok {
			return path
		}
	}
	return ""
}

func resolvePython(name string) (string, bool) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", false
	}
	return path, true
}

func hasCmd(name string) bool {
	_, err := exec.LookPath(name)
	return err != nil
}
