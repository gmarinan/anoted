package transcribe

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"anoted/internal/config"
)

var whisperCppNames = []string{"whisper-cli", "whisper-cpp", "whisper.cpp"}

func resolveBinary(cfg config.TranscriptionConfig) (path, backend string, err error) {
	// faster-whisper is a Python library, not a CLI, so it resolves to an
	// interpreter. This has to run before the configured-binary shortcut below,
	// which would otherwise hand back the openai-whisper CLI path.
	//
	// It is deliberately opt-in and never chosen by "auto": it needs the model
	// in CTranslate2 format, so selecting it silently would trigger a
	// multi-gigabyte download the user did not ask for.
	if strings.EqualFold(strings.TrimSpace(cfg.Backend), BackendFasterWhisper) {
		py := FasterWhisperPython(cfg)
		if py == "" {
			return "", "", fmt.Errorf("faster-whisper is not installed — run `anoted setup` or pip install faster-whisper into the whisper venv")
		}
		return py, BackendFasterWhisper, nil
	}
	// Same reasoning: a library behind an interpreter, opt-in only, and its
	// first run downloads a 2.4 GB model.
	if isParakeet(cfg) {
		py := ParakeetPython()
		if py == "" {
			return "", "", fmt.Errorf("parakeet is not installed — run `anoted parakeet install`")
		}
		return py, BackendParakeet, nil
	}

	if cfg.Binary != "" {
		if _, err := os.Stat(cfg.Binary); err != nil {
			return "", "", fmt.Errorf("transcription binary %q: %w", cfg.Binary, err)
		}
		return cfg.Binary, detectBackend(cfg.Binary, cfg.Backend), nil
	}

	want := strings.ToLower(cfg.Backend)
	if want == "" || want == BackendAuto {
		for _, name := range whisperCppNames {
			if p, err := exec.LookPath(name); err == nil {
				return p, BackendWhisperCpp, nil
			}
		}
		if ManagedWhisperInstalled() {
			return ManagedWhisperBinary(), BackendOpenAI, nil
		}
		if p, err := exec.LookPath("whisper"); err == nil {
			return p, BackendOpenAI, nil
		}
		return "", "", fmt.Errorf("whisper not found — run anoted setup or install openai-whisper")
	}

	switch want {
	case BackendWhisperCpp:
		for _, name := range whisperCppNames {
			if p, err := exec.LookPath(name); err == nil {
				return p, BackendWhisperCpp, nil
			}
		}
		return "", "", fmt.Errorf("whisper.cpp binary not found (whisper-cli / whisper-cpp)")
	case BackendOpenAI:
		if ManagedWhisperInstalled() {
			return ManagedWhisperBinary(), BackendOpenAI, nil
		}
		p, err := exec.LookPath("whisper")
		if err != nil {
			return "", "", fmt.Errorf("openai-whisper CLI not found (whisper)")
		}
		return p, BackendOpenAI, nil
	default:
		return "", "", fmt.Errorf("unknown transcription backend %q", cfg.Backend)
	}
}

func detectBackend(binPath, configured string) string {
	if configured != "" && configured != BackendAuto {
		return configured
	}
	base := strings.ToLower(filepath.Base(binPath))
	for _, name := range whisperCppNames {
		if base == name {
			return BackendWhisperCpp
		}
	}
	return BackendOpenAI
}

func resolveDevice(cfg config.TranscriptionConfig) string {
	switch strings.ToLower(cfg.Device) {
	case DeviceCUDA:
		return DeviceCUDA
	case DeviceCPU:
		return DeviceCPU
	default:
		// Parakeet runs on onnxruntime, not PyTorch: whether the Whisper venv
		// has a CUDA torch says nothing about it, and its GPU install brings
		// its own CUDA libraries, so a driver is all it needs.
		if isParakeet(cfg) {
			if NVIDIAAvailable() {
				return DeviceCUDA
			}
			return DeviceCPU
		}
		if cfg.GPULayers > 0 {
			return DeviceCUDA
		}
		if hasCUDA() {
			return DeviceCUDA
		}
		return DeviceCPU
	}
}

// hasCUDA reports whether transcription can actually run on the GPU.
//
// An NVIDIA driver alone is not enough: with a CPU-only torch wheel — which is
// what the managed venv installs by default — whisper raises "Torch not
// compiled with CUDA enabled" and fails outright. Requiring the torch probe too
// keeps `device: auto` from selecting a device that cannot work.
func hasCUDA() bool {
	if !NVIDIAAvailable() {
		return false
	}
	return ManagedTorchCUDAAvailable()
}

// ResolvedModel is the model a transcription with cfg will actually run.
func ResolvedModel(cfg config.TranscriptionConfig) string {
	return resolvedModel(cfg)
}

func resolvedModel(cfg config.TranscriptionConfig) string {
	if isParakeet(cfg) {
		return ParakeetModel
	}
	if cfg.Model != "" {
		return cfg.Model
	}
	return "turbo"
}
