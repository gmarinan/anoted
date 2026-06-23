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
}

// ConfigureTranscription applies transcription settings and optionally installs Whisper.
func ConfigureTranscription(cfg *config.Config, plan TranscriptionPlan, in io.Reader, out io.Writer, autoInstall bool) error {
	if plan.AutoAfterRecording {
		cfg.Transcription.AutoAfterRecording = true
	}
	if !plan.InstallWhisper {
		return nil
	}
	if transcribe.IsInstalled(*cfg) {
		path, _ := transcribe.BinaryPath(*cfg)
		fmt.Fprintf(out, "  ✓ Whisper already installed: %s\n", path)
		applyManagedWhisperCfg(cfg, path)
		return nil
	}
	if err := installTranscription(in, out, autoInstall); err != nil {
		return err
	}
	cfg.Transcription.Binary = transcribe.ManagedWhisperBinary()
	cfg.Transcription.Backend = transcribe.BackendOpenAI
	cfg.Transcription.Device = transcribe.DeviceCPU
	cfg.Transcription.GPULayers = 0
	return nil
}

func applyManagedWhisperCfg(cfg *config.Config, path string) {
	if transcribe.IsManagedBinary(path) {
		cfg.Transcription.Binary = path
		cfg.Transcription.Backend = transcribe.BackendOpenAI
	}
}
