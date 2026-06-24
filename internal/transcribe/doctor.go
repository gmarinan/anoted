package transcribe

import (
	"fmt"

	"anoted/internal/config"
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

	checks = append(checks, pythonCheck())

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
	gpu := DetectGPU()
	device := resolveDevice(cfg)
	usingCUDA := cfg.GPULayers > 0 || cfg.Device == DeviceCUDA || (device == DeviceCUDA && hasCUDA())

	if usingCUDA {
		if hasCUDA() {
			detail := formatGPUActiveDetail(gpu, cfg)
			return []Check{{Name: "transcription_gpu", Status: "ok", Detail: detail}}
		}
		return []Check{{Name: "transcription_gpu", Status: "warn", Detail: "CUDA requested but nvidia-smi not found; will use CPU"}}
	}

	if gpu.NVIDIA {
		if ManagedTorchCUDAAvailable() {
			return []Check{{Name: "transcription_gpu", Status: "ok", Detail: formatGPUActiveDetail(gpu, cfg)}}
		}
		return []Check{{Name: "transcription_gpu", Status: "ok", Detail: fmt.Sprintf("CPU (%s detected — press g in Doctor or Setup)", gpu.Name)}}
	}
	return []Check{{Name: "transcription_gpu", Status: "ok", Detail: "CPU"}}
}

func formatGPUActiveDetail(gpu GPUInfo, cfg config.TranscriptionConfig) string {
	if cfg.GPULayers > 0 {
		return fmt.Sprintf("CUDA (%d gpu_layers)", cfg.GPULayers)
	}
	if gpu.Name == "" {
		return "CUDA"
	}
	if gpu.Driver != "" {
		return fmt.Sprintf("CUDA — %s, driver %s", gpu.Name, gpu.Driver)
	}
	return fmt.Sprintf("CUDA — %s", gpu.Name)
}

func pythonCheck() Check {
	if py := discoverPython(); py != "" {
		return Check{Name: "python", Status: "ok", Detail: py}
	}
	return Check{Name: "python", Status: "warn", Detail: "not found — " + PythonInstallHint()}
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
