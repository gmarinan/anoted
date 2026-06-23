//go:build !linux && !windows

package setup

import (
	"io"

	"anoted/internal/transcribe"
)

func installTranscription(in io.Reader, out io.Writer, autoInstall bool) error {
	return transcribe.EnsureWhisperCaptured(out, autoInstall)
}
