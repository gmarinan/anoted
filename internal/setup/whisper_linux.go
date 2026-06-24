//go:build linux

package setup

import (
	"fmt"
	"io"
	"os"
	"os/exec"

	"anoted/internal/config"
	"anoted/internal/transcribe"
)

func setupTranscription(in io.Reader, out io.Writer, cfg *config.Config, autoInstall bool) {
	fmt.Fprintln(out, "[4/4] Transcription (Whisper)")
	fmt.Fprintln(out, "  ─────────────────────────────────────")
	fmt.Fprintln(out, "  Local speech-to-text → configurable outputs (txt, srt, md, …) per session")
	fmt.Fprintln(out)

	if askNo(in, out, "  Auto-transcribe after each recording? [y/N]: ") {
		cfg.Transcription.AutoAfterRecording = true
	}

	whisperReady := transcribe.IsInstalled(*cfg)
	if whisperReady {
		printWhisperStatus(out, cfg)
	} else {
		printWhisperInstallHints(out)
		prompt := "  Install Whisper in local venv (~500MB, no sudo)? [y/N]: "
		if autoInstall || askNo(in, out, prompt) {
			if err := installWhisper(out, autoInstall); err != nil {
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

	fmt.Fprintln(out)
	bin, _ := transcribe.BinaryPath(*cfg)
	if bin != "" {
		cfg.Transcription.Binary = bin
	}
	ConfigureGPUAfterWhisper(in, out, cfg, autoInstall)
	fmt.Fprintln(out)
}

func printWhisperStatus(out io.Writer, cfg *config.Config) {
	path, _ := transcribe.BinaryPath(*cfg)
	fmt.Fprintf(out, "  ✓ Whisper: %s\n", path)
	if transcribe.IsManagedBinary(path) {
		cfg.Transcription.Binary = path
		cfg.Transcription.Backend = transcribe.BackendOpenAI
	}
}

func printWhisperInstallHints(out io.Writer) {
	fmt.Fprintln(out, "  ⚠ Whisper not installed")
	fmt.Fprintf(out, "    → Recommended: local venv at %s\n", transcribe.ManagedVenvDir())
	if hint := transcribe.PacmanHint(); hint != "" {
		fmt.Fprintf(out, "    → Optional (system): %s\n", hint)
	}
	fmt.Fprintf(out, "    → Fast GPU path: %s\n", transcribe.WhisperCppHint())
}

func installWhisper(out io.Writer, autoInstall bool) error {
	if _, err := transcribe.FindPython(); err != nil {
		if autoInstall && hasCmd("pacman") {
			fmt.Fprintln(out, "  Python missing — installing via pacman…")
			cmd := exec.Command("sudo", "pacman", "-S", "--needed", "--noconfirm", "python")
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			fmt.Fprintf(out, "\n  Running: %s\n\n", joinCmd(cmd.Args))
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("install python: %w", err)
			}
		} else {
			return err
		}
	}
	return transcribe.EnsureWhisper(out, autoInstall)
}
