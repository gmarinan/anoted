//go:build linux

package transcribe

import (
	"testing"
)

func TestDetectGPU(t *testing.T) {
	gpu := DetectGPU()
	if findExecutable("nvidia-smi", "/usr/bin/nvidia-smi") == "" {
		if gpu.NVIDIA {
			t.Fatal("expected no NVIDIA GPU when nvidia-smi missing")
		}
		return
	}
	if !gpu.NVIDIA {
		t.Fatal("expected NVIDIA GPU when nvidia-smi present")
	}
	if gpu.Name == "" {
		t.Fatal("expected GPU name")
	}
}

func TestNVIDIAAvailableAbsolutePath(t *testing.T) {
	t.Setenv("PATH", "/bin")
	if findExecutable("nvidia-smi", "/usr/bin/nvidia-smi") == "" {
		t.Skip("nvidia-smi not at /usr/bin")
	}
	if !NVIDIAAvailable() {
		t.Fatal("expected NVIDIAAvailable via /usr/bin/nvidia-smi")
	}
}
