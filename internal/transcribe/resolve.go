package transcribe

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"meetctl/internal/config"
)

var whisperCppNames = []string{"whisper-cli", "whisper-cpp", "whisper.cpp"}

func resolveBinary(cfg config.TranscriptionConfig) (path, backend string, err error) {
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
		if p, err := exec.LookPath("whisper"); err == nil {
			return p, BackendOpenAI, nil
		}
		return "", "", fmt.Errorf("whisper not found in PATH (install openai-whisper or whisper.cpp)")
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
		if cfg.GPULayers > 0 {
			return DeviceCUDA
		}
		if hasCUDA() {
			return DeviceCUDA
		}
		return DeviceCPU
	}
}

func hasCUDA() bool {
	_, err := exec.LookPath("nvidia-smi")
	return err == nil
}

func resolvedModel(cfg config.TranscriptionConfig) string {
	if cfg.Model != "" {
		return cfg.Model
	}
	return "base"
}
