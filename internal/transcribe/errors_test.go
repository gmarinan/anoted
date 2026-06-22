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
	if !strings.Contains(got, "GPU out of memory") || !strings.Contains(got, "smaller model") {
		t.Fatalf("got %q", got)
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
