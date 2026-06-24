package transcribe

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"anoted/internal/config"
)

const torchCUDACacheTTL = 60 * time.Second

var torchCUDACache struct {
	sync.Mutex
	checked   bool
	available bool
	at        time.Time
}

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
	return installManaged(out, os.Stdout, os.Stderr)
}

func installManaged(progress, stdout, stderr io.Writer) error {
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
		fmt.Fprintf(progress, "  Creating venv at %s\n", venv)
		if err := runCmd(stdout, stderr, py, "-m", "venv", venv); err != nil {
			return formatCmdError("create venv", err)
		}
	}

	steps := []struct {
		desc string
		args []string
		soft bool
	}{
		{"Upgrading pip", pipInstallArgs(python, "-U", "pip"), true},
		{"Installing PyTorch (CPU)", pipInstallArgs(python, "install", "torch", "--index-url", "https://download.pytorch.org/whl/cpu"), false},
		{"Installing openai-whisper", pipInstallArgs(python, "install", "-U", "openai-whisper"), false},
	}
	for _, step := range steps {
		fmt.Fprintf(progress, "  %s…\n", step.desc)
		if err := runCmd(stdout, stderr, step.args...); err != nil {
			if step.soft {
				fmt.Fprintf(progress, "  ! pip upgrade skipped: %v\n", err)
				continue
			}
			return formatCmdError(step.desc, err)
		}
	}

	if !ManagedWhisperInstalled() {
		return fmt.Errorf("whisper binary missing after install")
	}
	fmt.Fprintf(progress, "  ✓ Installed: %s\n", ManagedWhisperBinary())
	return nil
}

// UpgradeManagedTorchCUDA replaces CPU PyTorch with a CUDA build in the managed venv.
func UpgradeManagedTorchCUDA(out io.Writer) error {
	return upgradeManagedTorchCUDA(out, os.Stdout, os.Stderr)
}

// UpgradeManagedTorchCUDACaptured routes pip output to the given writers (for TUI logs).
func UpgradeManagedTorchCUDACaptured(progress, stdout, stderr io.Writer) error {
	return upgradeManagedTorchCUDA(progress, stdout, stderr)
}

func upgradeManagedTorchCUDA(progress, stdout, stderr io.Writer) error {
	if !ManagedWhisperInstalled() {
		return fmt.Errorf("managed venv not installed — run anoted setup first")
	}
	python := venvPythonPath(resolveManagedVenvDir())
	indexes := []string{
		"https://download.pytorch.org/whl/cu126",
		"https://download.pytorch.org/whl/cu124",
		"https://download.pytorch.org/whl/cu121",
	}
	fmt.Fprintln(progress, "  Downloading PyTorch CUDA wheels (~1–2 GB, may take several minutes)…")
	var lastErr error
	for _, idx := range indexes {
		fmt.Fprintf(progress, "  Installing PyTorch (CUDA) from %s…\n", idx)
		args := pipInstallArgs(python, "install", "-U", "torch", "--index-url", idx)
		err := runCmd(stdout, stderr, args...)
		if err != nil {
			lastErr = err
			fmt.Fprintf(progress, "  ! index %s failed: %v\n", idx, err)
			continue
		}
		InvalidateTorchCUDACache()
		if ManagedTorchCUDAAvailable() {
			fmt.Fprintln(progress, "  ✓ PyTorch CUDA ready in managed venv")
			return nil
		}
		lastErr = fmt.Errorf("torch installed but CUDA not available")
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no CUDA torch wheel matched this system")
	}
	return lastErr
}

func pipInstallArgs(python string, pipArgs ...string) []string {
	args := []string{python, "-m", "pip", "install", "--progress-bar", "on", "-v"}
	return append(args, pipArgs...)
}

// ManagedTorchCUDAAvailable reports whether the managed venv can use CUDA.
func ManagedTorchCUDAAvailable() bool {
	torchCUDACache.Lock()
	if torchCUDACache.checked && time.Since(torchCUDACache.at) < torchCUDACacheTTL {
		available := torchCUDACache.available
		torchCUDACache.Unlock()
		return available
	}
	torchCUDACache.Unlock()

	available := probeManagedTorchCUDA()
	setTorchCUDACache(available)
	return available
}

// InvalidateTorchCUDACache clears the CUDA availability probe cache.
func InvalidateTorchCUDACache() {
	torchCUDACache.Lock()
	torchCUDACache.checked = false
	torchCUDACache.Unlock()
}

func setTorchCUDACache(available bool) {
	torchCUDACache.Lock()
	torchCUDACache.checked = true
	torchCUDACache.available = available
	torchCUDACache.at = time.Now()
	torchCUDACache.Unlock()
}

func probeManagedTorchCUDA() bool {
	python := venvPythonPath(resolveManagedVenvDir())
	cmd := exec.Command(python, "-c", "import torch; raise SystemExit(0 if torch.cuda.is_available() else 1)")
	return cmd.Run() == nil
}

func runCmd(stdout, stderr io.Writer, args ...string) error {
	if len(args) == 0 {
		return fmt.Errorf("empty command")
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

func formatCmdError(step string, err error) error {
	if err == nil {
		return nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && runtime.GOOS == "windows" && exitErr.ExitCode() == 9009 {
		return fmt.Errorf("%s: command not found (exit 9009) — install Python and ensure it is on PATH, or run: %s", step, PythonInstallHint())
	}
	return fmt.Errorf("%s: %w", step, err)
}

func findPython() (string, error) {
	if py := discoverPython(); py != "" {
		return py, nil
	}
	return "", fmt.Errorf("python not found — %s", PythonInstallHint())
}

// FindPython returns the resolved Python interpreter path.
func FindPython() (string, error) {
	return findPython()
}

func hasCmd(name string) bool {
	_, err := exec.LookPath(name)
	return err != nil
}
