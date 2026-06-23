//go:build linux

package setup

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

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
	configureTranscriptionDevice(in, out, cfg, bin)
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

func configureTranscriptionDevice(in io.Reader, out io.Writer, cfg *config.Config, bin string) {
	gpu := transcribe.DetectGPU()
	fmt.Fprintln(out, "  Hardware")
	fmt.Fprintln(out, "  ────────")
	printGPUStatus(out, gpu, bin)

	if !gpu.NVIDIA {
		cfg.Transcription.Device = transcribe.DeviceCPU
		cfg.Transcription.GPULayers = 0
		fmt.Fprintln(out)
		fmt.Fprintln(out, "  ○ Using CPU (no NVIDIA GPU detected)")
		return
	}

	fmt.Fprintln(out)
	fmt.Fprintln(out, "  → GPU speeds up Whisper significantly (recommended on NVIDIA)")
	if transcribe.IsManagedBinary(bin) && !transcribe.ManagedTorchCUDAAvailable() {
		fmt.Fprintln(out, "  ℹ Enabling GPU downloads CUDA PyTorch wheels (~1–2 GB) into the venv")
	} else if transcribe.IsManagedBinary(bin) && transcribe.ManagedTorchCUDAAvailable() {
		cfg.Transcription.Device = transcribe.DeviceCUDA
		cfg.Transcription.GPULayers = 0
		fmt.Fprintln(out)
		fmt.Fprintln(out, "  ✓ GPU already enabled in managed venv")
		return
	}

	if !askYes(in, out, "  Use GPU (CUDA) for transcription? [Y/n]: ") {
		cfg.Transcription.Device = transcribe.DeviceCPU
		cfg.Transcription.GPULayers = 0
		fmt.Fprintln(out)
		fmt.Fprintln(out, "  ○ Using CPU for transcription")
		return
	}

	if transcribe.IsManagedBinary(bin) {
		fmt.Fprintln(out)
		fmt.Fprintln(out, "  Upgrading venv PyTorch to CUDA…")
		if err := transcribe.UpgradeManagedTorchCUDA(out); err != nil {
			fmt.Fprintf(out, "  ⚠ GPU setup failed: %v\n", err)
			cfg.Transcription.Device = transcribe.DeviceCPU
			cfg.Transcription.GPULayers = 0
			fmt.Fprintln(out, "  ○ Falling back to CPU")
			return
		}
		cfg.Transcription.Device = transcribe.DeviceCUDA
		cfg.Transcription.GPULayers = 0
		fmt.Fprintln(out)
		fmt.Fprintln(out, "  ✓ GPU enabled (PyTorch CUDA in venv)")
		return
	}

	cfg.Transcription.Device = transcribe.DeviceCUDA
	cfg.Transcription.GPULayers = 99
	fmt.Fprintln(out)
	fmt.Fprintln(out, "  ✓ GPU enabled (transcription.device: cuda)")
}

func printGPUStatus(out io.Writer, gpu transcribe.GPUInfo, bin string) {
	if gpu.NVIDIA {
		line := fmt.Sprintf("  ✓ GPU: %s", gpu.Name)
		var details []string
		if gpu.Driver != "" {
			details = append(details, "driver "+gpu.Driver)
		}
		if gpu.CUDAVersion != "" {
			details = append(details, "CUDA "+gpu.CUDAVersion)
		}
		if len(details) > 0 {
			line += " (" + strings.Join(details, ", ") + ")"
		}
		fmt.Fprintln(out, line)
		if transcribe.IsManagedBinary(bin) && transcribe.ManagedTorchCUDAAvailable() {
			fmt.Fprintln(out, "  ✓ PyTorch CUDA: ready in managed venv")
		} else if transcribe.IsManagedBinary(bin) {
			fmt.Fprintln(out, "  ○ PyTorch: CPU build in managed venv (GPU available to enable)")
		}
		return
	}
	fmt.Fprintln(out, "  ○ GPU: none detected (Whisper will use CPU)")
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
