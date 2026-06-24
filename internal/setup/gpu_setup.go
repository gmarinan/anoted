package setup

import (
	"fmt"
	"io"

	"anoted/internal/config"
	"anoted/internal/transcribe"
)

// GPUPlan holds GPU setup choices.
type GPUPlan struct {
	Enable bool
}

// GPUOfferAvailable reports whether GPU enablement can be offered (NVIDIA + managed venv).
func GPUOfferAvailable(cfg config.Config) bool {
	if !transcribe.DetectGPU().NVIDIA {
		return false
	}
	if !transcribe.IsInstalled(cfg) {
		return false
	}
	bin, err := transcribe.BinaryPath(cfg)
	if err != nil || !transcribe.IsManagedBinary(bin) {
		return false
	}
	return !transcribe.ManagedTorchCUDAAvailable()
}

// ConfigureGPU upgrades managed PyTorch to CUDA when requested.
func ConfigureGPU(cfg *config.Config, out io.Writer, enable bool) error {
	if !enable {
		return nil
	}
	gpu := transcribe.DetectGPU()
	if !gpu.NVIDIA {
		return fmt.Errorf("no NVIDIA GPU detected")
	}
	if !transcribe.ManagedWhisperInstalled() {
		return fmt.Errorf("managed whisper venv not installed")
	}
	if transcribe.ManagedTorchCUDAAvailable() {
		cfg.Transcription.Device = transcribe.DeviceCUDA
		cfg.Transcription.GPULayers = 0
		return nil
	}
	if err := transcribe.UpgradeManagedTorchCUDA(out); err != nil {
		return err
	}
	cfg.Transcription.Device = transcribe.DeviceCUDA
	cfg.Transcription.GPULayers = 0
	return nil
}

// ConfigureGPUAfterWhisper prompts for GPU on CLI setup (mirrors Linux flow).
func ConfigureGPUAfterWhisper(in io.Reader, out io.Writer, cfg *config.Config, autoInstall bool) {
	gpu := transcribe.DetectGPU()
	fmt.Fprintln(out)
	fmt.Fprintln(out, "  Hardware")
	fmt.Fprintln(out, "  ────────")
	printGPUStatus(out, gpu, cfg.Transcription.Binary)

	if !gpu.NVIDIA {
		cfg.Transcription.Device = transcribe.DeviceCPU
		cfg.Transcription.GPULayers = 0
		fmt.Fprintln(out)
		fmt.Fprintln(out, "  ○ Using CPU (no NVIDIA GPU detected)")
		return
	}

	bin := cfg.Transcription.Binary
	if bin == "" {
		bin, _ = transcribe.BinaryPath(*cfg)
	}
	if !transcribe.IsManagedBinary(bin) {
		fmt.Fprintln(out)
		fmt.Fprintln(out, "  → GPU via whisper.cpp (non-managed binary)")
		if !autoInstall && !askYes(in, out, "  Use GPU for transcription? [Y/n]: ") {
			cfg.Transcription.Device = transcribe.DeviceCPU
			cfg.Transcription.GPULayers = 0
			fmt.Fprintln(out)
			fmt.Fprintln(out, "  ○ Using CPU for transcription")
			return
		}
		cfg.Transcription.Device = transcribe.DeviceCUDA
		cfg.Transcription.GPULayers = 99
		fmt.Fprintln(out)
		fmt.Fprintln(out, "  ✓ GPU enabled (transcription.device: cuda)")
		return
	}

	if transcribe.ManagedTorchCUDAAvailable() {
		cfg.Transcription.Device = transcribe.DeviceCUDA
		cfg.Transcription.GPULayers = 0
		fmt.Fprintln(out)
		fmt.Fprintln(out, "  ✓ GPU already enabled in managed venv")
		return
	}

	fmt.Fprintln(out)
	fmt.Fprintln(out, "  → GPU speeds up Whisper significantly (recommended on NVIDIA)")
	fmt.Fprintln(out, "  ℹ Enabling GPU downloads CUDA PyTorch wheels (~1–2 GB) into the venv")
	if !autoInstall && !askYes(in, out, "  Use GPU (CUDA) for transcription? [Y/n]: ") {
		cfg.Transcription.Device = transcribe.DeviceCPU
		cfg.Transcription.GPULayers = 0
		fmt.Fprintln(out)
		fmt.Fprintln(out, "  ○ Using CPU for transcription")
		return
	}

	if err := ConfigureGPU(cfg, out, true); err != nil {
		fmt.Fprintf(out, "  ⚠ GPU setup failed: %v\n", err)
		cfg.Transcription.Device = transcribe.DeviceCPU
		cfg.Transcription.GPULayers = 0
		fmt.Fprintln(out, "  ○ Falling back to CPU")
		return
	}
	fmt.Fprintln(out)
	fmt.Fprintln(out, "  ✓ GPU enabled (PyTorch CUDA in venv)")
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
			line += " (" + joinDetails(details) + ")"
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

func joinDetails(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	out := parts[0]
	for i := 1; i < len(parts); i++ {
		out += ", " + parts[i]
	}
	return out
}
