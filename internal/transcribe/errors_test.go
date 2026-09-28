package transcribe

import (
	"strings"
	"testing"
)

func TestExtractWhisperError_OOM(t *testing.T) {
	out := `Traceback (most recent call last):
  File "whisper", line 6, in <module>
torch.OutOfMemoryError: CUDA out of memory. Tried to allocate 20.00 MiB.`
	got := extractWhisperError(out)
	if got == "" {
		t.Fatal("expected message")
	}
	if !strings.Contains(got, "GPU out of memory") || !strings.Contains(got, "smaller model") || !strings.Contains(got, "close other apps") {
		t.Fatalf("got %q", got)
	}
}

func TestIsCUDAOOM(t *testing.T) {
	oom := []string{
		"torch.OutOfMemoryError: CUDA out of memory. Tried to allocate 20.00 MiB.",
		"CUDA failed with error out of memory",
		"ggml_cuda_error: cudaMalloc failed: out of memory",
		"GPU out of memory — close other apps using the GPU",
	}
	for _, s := range oom {
		if !isCUDAOOM(s) {
			t.Errorf("expected OOM: %s", s)
		}
	}
	not := []string{
		"file not found",
		"out of memory", // system RAM, no GPU signal
		"Torch not compiled with CUDA enabled",
		"signal: aborted",
	}
	for _, s := range not {
		if isCUDAOOM(s) {
			t.Errorf("expected not OOM: %s", s)
		}
	}
}

func TestIsCUDAFailure(t *testing.T) {
	if !isCUDAFailure([]byte("torch.OutOfMemoryError: CUDA out of memory")) {
		t.Fatal("expected cuda failure")
	}
	if isCUDAFailure([]byte("file not found")) {
		t.Fatal("unexpected cuda failure")
	}
}
