//go:build windows

package setup

import (
	"fmt"
	"io"

	"anoted/internal/config"
	"anoted/internal/transcribe"
)

func setupTranscription(in io.Reader, out io.Writer, cfg *config.Config, autoInstall bool) {
	fmt.Fprintln(out, "[4/4] Transcription (Whisper)")
	fmt.Fprintln(out, "  ─────────────────────────────────────")
	fmt.Fprintln(out, "  Local speech-to-text → transcript.txt + .srt per session")
	fmt.Fprintln(out)

	if askNo(in, out, "  Auto-transcribe after each recording? [y/N]: ") {
		cfg.Transcription.AutoAfterRecording = true
	}

	whisperReady := transcribe.IsInstalled(*cfg)
	if whisperReady {
		path, _ := transcribe.BinaryPath(*cfg)
		fmt.Fprintf(out, "  ✓ Whisper: %s\n", path)
		if transcribe.IsManagedBinary(path) {
			cfg.Transcription.Binary = path
			cfg.Transcription.Backend = transcribe.BackendOpenAI
		}
	} else {
		fmt.Fprintln(out, "  ⚠ Whisper not installed")
		fmt.Fprintf(out, "    → Recommended: local venv at %s\n", transcribe.ManagedVenvDir())
		fmt.Fprintf(out, "    → Install Python: %s\n", transcribe.PythonInstallHint())
		prompt := "  Install Whisper in local venv (~500MB)? [y/N]: "
		if autoInstall || askNo(in, out, prompt) {
			if _, err := transcribe.FindPython(); err != nil {
				fmt.Fprintf(out, "  ⚠ %v\n", err)
			} else if err := transcribe.InstallManaged(out); err != nil {
				fmt.Fprintf(out, "  ⚠ Install failed: %v\n", err)
				fmt.Fprintf(out, "    Retry: %s\n", transcribe.InstallHint())
			} else {
				cfg.Transcription.Binary = transcribe.ManagedWhisperBinary()
				cfg.Transcription.Backend = transcribe.BackendOpenAI
				whisperReady = true
				fmt.Fprintf(out, "  ✓ Whisper ready: %s\n", cfg.Transcription.Binary)
			}
		}
	}

	if !whisperReady {
		fmt.Fprintln(out)
		fmt.Fprintln(out, "  ○ Transcription skipped — run anoted setup again later")
		fmt.Fprintln(out)
		return
	}

	cfg.Transcription.Device = transcribe.DeviceCPU
	cfg.Transcription.GPULayers = 0
	fmt.Fprintln(out)
	fmt.Fprintln(out, "  ○ Using CPU for transcription on Windows")
	fmt.Fprintln(out)
}
