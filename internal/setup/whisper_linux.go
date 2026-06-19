//go:build linux

package setup

import (
	"fmt"
	"io"
	"os"
	"os/exec"

	"meetctl/internal/config"
	"meetctl/internal/transcribe"
)

func setupTranscription(in io.Reader, out io.Writer, cfg *config.Config, autoInstall bool) {
	fmt.Fprintln(out, "[4/4] Transcription (Whisper, optional)")
	fmt.Fprintln(out, "  Transcribe recordings locally — output saved as transcript.txt in each session folder.")
	fmt.Fprintln(out)

	if askYes(in, out, "  Enable auto-transcription after each recording? [y/N]: ") {
		cfg.Transcription.AutoAfterRecording = true
	}

	if transcribe.IsInstalled(*cfg) {
		path, _ := transcribe.BinaryPath(*cfg)
		fmt.Fprintf(out, "  ✓ Whisper found: %s\n", path)
	} else {
		fmt.Fprintln(out, "  ⚠ Whisper not installed")
		fmt.Fprintf(out, "    Install hint: %s\n", transcribe.InstallHint())
		if autoInstall || askYes(in, out, "  Try installing whisper.cpp now? (needs sudo) [y/N]: ") {
			if err := installWhisperCpp(out); err != nil {
				fmt.Fprintf(out, "  ⚠ Install failed: %v\n", err)
			} else if transcribe.IsInstalled(*cfg) {
				path, _ := transcribe.BinaryPath(*cfg)
				fmt.Fprintf(out, "  ✓ Whisper ready: %s\n", path)
			}
		}
	}

	if hasCUDA() && askYes(in, out, "  Use GPU (CUDA) for transcription? [y/N]: ") {
		cfg.Transcription.Device = transcribe.DeviceCUDA
		cfg.Transcription.GPULayers = 99
		fmt.Fprintln(out, "  ✓ GPU enabled (transcription.device: cuda, gpu_layers: 99)")
	} else {
		cfg.Transcription.Device = transcribe.DeviceCPU
		fmt.Fprintln(out, "  ○ Using CPU for transcription")
	}
	fmt.Fprintln(out)
}

func installWhisperCpp(out io.Writer) error {
	if hasCmd("pacman") {
		cmd := exec.Command("sudo", "pacman", "-S", "--needed", "--noconfirm", "whisper.cpp")
		fmt.Fprintf(out, "\n  Running: %s\n\n", joinCmd(cmd.Args))
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	return fmt.Errorf("automatic install only supported on pacman systems — %s", transcribe.InstallHint())
}

func hasCUDA() bool {
	_, err := exec.LookPath("nvidia-smi")
	return err == nil
}
