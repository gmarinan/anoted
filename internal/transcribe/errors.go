package transcribe

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// formatWhisperError turns subprocess output into a short actionable message.
//
// ctx is consulted first because exec.CommandContext kills the child on
// cancellation and Wait then reports the signal ("signal: killed") rather than
// context.Canceled — so a user pressing stop saw a fragment of their own
// transcript rendered as a red error on the session row.
func formatWhisperError(ctx context.Context, backend string, out []byte, err error) error {
	if ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil && errors.Is(err, context.Canceled) {
		return context.Canceled
	}
	msg := extractWhisperError(string(out))
	if msg == "" && err != nil {
		msg = err.Error()
	}
	if msg == "" {
		msg = "transcription failed"
	}
	return fmt.Errorf("%s: %s", backend, msg)
}

func extractWhisperError(out string) string {
	lines := strings.Split(out, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "Traceback") || strings.HasPrefix(line, "File ") {
			continue
		}
		if strings.Contains(line, "torch.OutOfMemoryError") || strings.Contains(line, "CUDA out of memory") {
			return "GPU out of memory — try a smaller model (turbo/base) or set transcription.device: cpu"
		}
		if strings.Contains(line, "No module named") {
			return line
		}
		if strings.HasPrefix(line, "Error:") || strings.Contains(line, "Error:") {
			return line
		}
		// Last non-traceback line in a Python exception often has the real error.
		if !strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "^") {
			return line
		}
	}
	return strings.TrimSpace(out)
}

func isCUDAFailure(out []byte) bool {
	s := string(out)
	return strings.Contains(s, "CUDA out of memory") ||
		strings.Contains(s, "torch.OutOfMemoryError") ||
		strings.Contains(s, "CUDA error") ||
		strings.Contains(s, "no kernel image is available") ||
		strings.Contains(s, "Found no NVIDIA driver") ||
		// A CPU-only torch build against an NVIDIA machine: recoverable by
		// retrying on CPU, but it does not mention "CUDA error".
		strings.Contains(s, "Torch not compiled with CUDA enabled") ||
		strings.Contains(s, "torch.cuda.is_available() is False")
}
