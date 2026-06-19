package transcribe

import (
	"fmt"
	"os/exec"

	"meetctl/internal/config"
)

// Check is a doctor-style diagnostic row.
type Check struct {
	Name   string
	Status string
	Detail string
}

// DoctorChecks validates Whisper installation and GPU settings.
func DoctorChecks(cfg config.Config) []Check {
	tcfg := cfg.Transcription
	var checks []Check

	bin, backend, err := resolveBinary(tcfg)
	if err != nil {
		checks = append(checks, Check{
			Name:   "whisper",
			Status: "warn",
			Detail: err.Error(),
		})
		return append(checks, gpuCheck(tcfg)...)
	}

	detail := fmt.Sprintf("%s (%s)", bin, backend)
	if backend == BackendWhisperCpp {
		if _, err := resolveCppModelPath(tcfg); err != nil {
			checks = append(checks, Check{
				Name:   "whisper",
				Status: "warn",
				Detail: detail + " — " + err.Error(),
			})
		} else {
			checks = append(checks, Check{Name: "whisper", Status: "ok", Detail: detail})
		}
	} else {
		checks = append(checks, Check{Name: "whisper", Status: "ok", Detail: detail})
	}

	checks = append(checks, Check{
		Name:   "transcription_model",
		Status: "ok",
		Detail: resolvedModel(tcfg),
	})

	if tcfg.AutoAfterRecording {
		checks = append(checks, Check{
			Name:   "auto_transcription",
			Status: "ok",
			Detail: "enabled after each recording",
		})
	} else {
		checks = append(checks, Check{
			Name:   "auto_transcription",
			Status: "ok",
			Detail: "disabled (manual in Sessions tab)",
		})
	}

	checks = append(checks, gpuCheck(tcfg)...)
	return checks
}

func gpuCheck(cfg config.TranscriptionConfig) []Check {
	device := resolveDevice(cfg)
	if cfg.GPULayers > 0 || cfg.Device == DeviceCUDA {
		if hasCUDA() {
			if cfg.GPULayers > 0 {
				return []Check{{Name: "transcription_gpu", Status: "ok", Detail: fmt.Sprintf("CUDA (%d gpu_layers)", cfg.GPULayers)}}
			}
			return []Check{{Name: "transcription_gpu", Status: "ok", Detail: "CUDA available"}}
		}
		return []Check{{Name: "transcription_gpu", Status: "warn", Detail: "CUDA requested but nvidia-smi not found; will use CPU"}}
	}
	if device == DeviceCUDA && hasCUDA() {
		return []Check{{Name: "transcription_gpu", Status: "ok", Detail: "auto → CUDA"}}
	}
	return []Check{{Name: "transcription_gpu", Status: "ok", Detail: "CPU"}}
}

// Verify runs a lightweight availability check.
func Verify(cfg config.Config) error {
	_, _, err := resolveBinary(cfg.Transcription)
	return err
}

// BinaryPath returns the resolved whisper binary if available.
func BinaryPath(cfg config.Config) (string, error) {
	p, _, err := resolveBinary(cfg.Transcription)
	return p, err
}

// IsInstalled reports whether a whisper backend is available.
func IsInstalled(cfg config.Config) bool {
	return Verify(cfg) == nil
}

// InstallHint returns a suggested install command for the current OS.
func InstallHint() string {
	if _, err := exec.LookPath("pacman"); err == nil {
		return "sudo pacman -S whisper.cpp  # or: pip install -U openai-whisper"
	}
	if _, err := exec.LookPath("apt-get"); err == nil {
		return "pip install -U openai-whisper  # or build whisper.cpp from source"
	}
	return "pip install -U openai-whisper  # or install whisper.cpp (whisper-cli)"
}
