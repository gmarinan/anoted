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
	fmt.Fprintln(out, "  Local speech-to-text → configurable outputs (txt, srt, md, …) per session")
	fmt.Fprintln(out)

	plan := TranscriptionPlan{}
	if askNo(in, out, "  Auto-transcribe after each recording? [y/N]: ") {
		plan.AutoAfterRecording = true
	}

	whisperReady := transcribe.IsInstalled(*cfg)
	if whisperReady {
		path, _ := transcribe.BinaryPath(*cfg)
		fmt.Fprintf(out, "  ✓ Whisper: %s\n", path)
		applyManagedWhisperCfg(cfg, path)
	} else {
		printWhisperInstallHints(out)
		prompt := "  Install Whisper in local venv (~500MB)? [y/N]: "
		if autoInstall || askNo(in, out, prompt) {
			plan.InstallWhisper = true
			if err := ConfigureTranscription(cfg, plan, in, out, autoInstall); err != nil {
				fmt.Fprintf(out, "  ⚠ Install failed: %v\n", err)
				fmt.Fprintf(out, "    Retry: %s\n", transcribe.InstallHint())
				fmt.Fprintln(out, "    Or install from the TUI: Doctor tab → i, or open Setup (S)")
			} else if plan.InstallWhisper {
				whisperReady = true
				fmt.Fprintf(out, "  ✓ Whisper ready: %s\n", cfg.Transcription.Binary)
			}
		}
		if !whisperReady && plan.AutoAfterRecording {
			cfg.Transcription.AutoAfterRecording = true
		}
	}

	if !whisperReady {
		fmt.Fprintln(out)
		fmt.Fprintln(out, "  ○ Transcription skipped — run anoted setup again or open Setup in the TUI")
		fmt.Fprintln(out)
		return
	}

	ConfigureGPUAfterWhisper(in, out, cfg, autoInstall)
}

func printWhisperInstallHints(out io.Writer) {
	fmt.Fprintln(out, "  ⚠ Whisper not installed")
	fmt.Fprintf(out, "    → Recommended: local venv at %s\n", transcribe.ManagedVenvDir())
	if _, err := transcribe.FindPython(); err != nil {
		fmt.Fprintf(out, "    → %v\n", err)
		if hasCmd("winget") {
			fmt.Fprintln(out, "    → Setup can install Python automatically via winget")
		}
	}
}
