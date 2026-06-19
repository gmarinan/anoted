package transcribe

import (
	"testing"

	"meetctl/internal/config"
)

func TestDetectBackend(t *testing.T) {
	if got := detectBackend("/usr/bin/whisper-cli", BackendAuto); got != BackendWhisperCpp {
		t.Fatalf("got %q", got)
	}
	if got := detectBackend("/usr/bin/whisper", BackendAuto); got != BackendOpenAI {
		t.Fatalf("got %q", got)
	}
}

func TestResolvedModelDefault(t *testing.T) {
	if got := resolvedModel(config.TranscriptionConfig{}); got != "turbo" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveDeviceCPU(t *testing.T) {
	cfg := config.TranscriptionConfig{Device: DeviceCPU, GPULayers: 0}
	if got := resolveDevice(cfg); got != DeviceCPU {
		t.Fatalf("got %q", got)
	}
}
