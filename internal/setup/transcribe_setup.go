package setup

import (
	"fmt"
	"io"

	"anoted/internal/config"
	"anoted/internal/transcribe"
)

// TranscriptionPlan holds transcription choices from setup.
type TranscriptionPlan struct {
	AutoAfterRecording bool
	InstallWhisper     bool
	EnableGPU          bool
}

// ConfigureTranscription applies transcription settings and optionally installs Whisper.
func ConfigureTranscription(cfg *config.Config, plan TranscriptionPlan, in io.Reader, out io.Writer, autoInstall bool) error {
	if plan.AutoAfterRecording {
		cfg.Transcription.AutoAfterRecording = true
	}
	if plan.EnableGPU && !transcribe.IsInstalled(*cfg) {
		plan.InstallWhisper = true
	}
	if !plan.InstallWhisper && !plan.EnableGPU {
		return nil
	}
	if plan.InstallWhisper {
		if transcribe.IsInstalled(*cfg) {
			path, _ := transcribe.BinaryPath(*cfg)
			fmt.Fprintf(out, "  ✓ Whisper already installed: %s\n", path)
			applyManagedWhisperCfg(cfg, path)
		} else if err := installTranscription(in, out, autoInstall); err != nil {
			return err
		} else {
			cfg.Transcription.Binary = transcribe.ManagedWhisperBinary()
			cfg.Transcription.Backend = transcribe.BackendOpenAI
			cfg.Transcription.Device = transcribe.DeviceCPU
			cfg.Transcription.GPULayers = 0
		}
	}
	if plan.EnableGPU {
		return ConfigureGPU(cfg, out, true)
	}
	return nil
}

func applyManagedWhisperCfg(cfg *config.Config, path string) {
	if transcribe.IsManagedBinary(path) {
		cfg.Transcription.Binary = path
		cfg.Transcription.Backend = transcribe.BackendOpenAI
	}
}
