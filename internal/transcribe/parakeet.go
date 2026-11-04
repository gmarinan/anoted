package transcribe

import (
	"context"
	_ "embed"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"anoted/internal/config"
)

//go:embed parakeet_runner.py
var parakeetRunner string

const (
	// ParakeetModel is the one model the parakeet backend runs; the
	// transcription.model setting belongs to the Whisper backends.
	ParakeetModel = "parakeet-tdt-0.6b-v3"

	parakeetVenvName = "parakeet-venv"
	parakeetModelDir = "parakeet-tdt-0.6b-v3-onnx"
	parakeetImports  = "onnx_asr, onnxruntime, numpy"
)

// parakeetModelFiles must match MODEL_FILES in parakeet_runner.py.
var parakeetModelFiles = []string{
	"config.json",
	"vocab.txt",
	"encoder-model.onnx",
	"encoder-model.onnx.data",
	"decoder_joint-model.onnx",
}

func isParakeet(cfg config.TranscriptionConfig) bool {
	return strings.EqualFold(strings.TrimSpace(cfg.Backend), BackendParakeet)
}

// ParakeetVenvDir is the anoted-managed venv for the parakeet backend.
//
// It is not the Whisper venv on purpose: faster-whisper depends on the CPU
// onnxruntime package and parakeet needs onnxruntime-gpu. Both install the
// same `onnxruntime` module, so in one venv whichever came last would win.
func ParakeetVenvDir() string {
	return filepath.Join(filepath.Dir(ManagedVenvDir()), parakeetVenvName)
}

// ParakeetModelDir holds the ONNX export as plain files, not in the Hugging
// Face cache: the runner explains why.
func ParakeetModelDir() string {
	return filepath.Join(filepath.Dir(ManagedVenvDir()), "models", parakeetModelDir)
}

// ParakeetPython returns the parakeet venv's interpreter when its runtime
// imports, or "" when the backend is not installed.
func ParakeetPython() string {
	py := venvPythonPath(ParakeetVenvDir())
	if _, err := os.Stat(py); err != nil {
		return ""
	}
	if !pythonCanImport(py, parakeetImports) {
		return ""
	}
	return py
}

// ParakeetModelDownloaded reports whether every model file is on disk. The
// runner downloads a missing model itself, so this only decides whether the
// first transcription will start with a 2.4 GB download.
func ParakeetModelDownloaded() bool {
	dir := ParakeetModelDir()
	for _, f := range parakeetModelFiles {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			return false
		}
	}
	return true
}

// InstallParakeet creates the parakeet venv, installs onnx-asr with the
// onnxruntime build that fits this machine, and downloads the model.
//
// With an NVIDIA GPU that is onnxruntime-gpu plus CUDA and cuDNN as pip wheels
// (~2.5 GB with the model, no system CUDA toolkit needed); without one, the
// CPU build. No root required.
func InstallParakeet(progress, stdout, stderr io.Writer) error {
	py, err := findPython()
	if err != nil {
		return err
	}
	venv := ParakeetVenvDir()
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

	packages, desc := parakeetPackages(NVIDIAAvailable())
	steps := []struct {
		desc string
		args []string
		soft bool
	}{
		{"Upgrading pip", pipInstallArgs(python, "-U", "pip"), true},
		{desc, pipInstallArgs(python, append([]string{"-U"}, packages...)...), false},
	}
	for _, step := range steps {
		fmt.Fprintf(progress, "  %s…\n", step.desc)
		if err := runCmd(stdout, stderr, step.args...); err != nil {
			if step.soft {
				fmt.Fprintf(progress, "  ! %s skipped: %v\n", step.desc, err)
				continue
			}
			return formatCmdError(step.desc, err)
		}
	}
	invalidateImportProbes()
	if ParakeetPython() == "" {
		return fmt.Errorf("parakeet runtime installed but %s does not import it", python)
	}

	if !ParakeetModelDownloaded() {
		fmt.Fprintf(progress, "  Downloading %s (~2.4 GB) to %s…\n", ParakeetModel, ParakeetModelDir())
		if err := downloadParakeetModel(python, stdout, stderr); err != nil {
			return formatCmdError("download model", err)
		}
		if !ParakeetModelDownloaded() {
			return fmt.Errorf("model download finished but files are missing in %s", ParakeetModelDir())
		}
	}
	fmt.Fprintf(progress, "  ✓ Parakeet ready: %s\n", python)
	return nil
}

// parakeetPackages picks the pip requirements for this machine.
func parakeetPackages(nvidia bool) (pkgs []string, desc string) {
	if nvidia {
		// onnxruntime-gpu ships without CUDA and cuDNN; its extras bring them
		// as wheels, and the runner loads them with preload_dlls().
		return []string{"onnx-asr[gpu,hub]", "onnxruntime-gpu[cuda,cudnn]"},
			"Installing onnx-asr + onnxruntime-gpu with CUDA/cuDNN (~2 GB)"
	}
	return []string{"onnx-asr[cpu,hub]"}, "Installing onnx-asr + onnxruntime (CPU)"
}

// downloadParakeetModel runs the runner in --download-only mode, so the
// download logic lives in one place.
//
//nolint:contextcheck // installer steps run to completion; there is no caller ctx to honour
func downloadParakeetModel(python string, stdout, stderr io.Writer) error {
	cmd := exec.CommandContext(context.Background(), python, "-", "--model-dir", ParakeetModelDir(), "--download-only")
	cmd.Stdin = strings.NewReader(parakeetRunner)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run %s: %w", python, err)
	}
	return nil
}

func transcribeParakeet(ctx context.Context, cfg config.TranscriptionConfig, python, audioPath, outDir, sessionDir string, onProgress ProgressFunc) (Result, error) {
	device := resolveDevice(cfg)
	if !ParakeetModelDownloaded() {
		emitProgress(onProgress, Progress{SegmentText: "· downloading the Parakeet model (~2.4 GB, first run only)…"})
	}
	segs, language, err := runJSONRunner(ctx, "parakeet", python, parakeetRunner, parakeetArgs(cfg, audioPath, device), onProgress)
	if err != nil {
		return Result{}, err
	}
	return writeRunnerSegments(cfg, outDir, sessionDir, segs, language, "parakeet")
}

func parakeetArgs(cfg config.TranscriptionConfig, audioPath, device string) []string {
	args := []string{
		"-", // read the runner from stdin
		"--audio", audioPath,
		"--model-dir", ParakeetModelDir(),
		"--device", device,
	}
	// Parakeet cannot be told the language. The runner uses it to decide
	// whether a passage that came out in English is a drift to undo.
	if lang := strings.TrimSpace(cfg.Language); lang != "" {
		args = append(args, "--language", lang)
	}
	return args
}
