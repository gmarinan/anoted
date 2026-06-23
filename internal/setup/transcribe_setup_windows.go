//go:build windows

package setup

import (
	"io"

	"anoted/internal/transcribe"
)

func installTranscription(in io.Reader, out io.Writer, autoInstall bool) error {
	if _, err := transcribe.FindPython(); err != nil {
		installPython := autoInstall
		if !installPython {
			installPython = askYes(in, out, "  Python not found — install via winget? [Y/n]: ")
		}
		if !installPython {
			return err
		}
	}
	return transcribe.EnsureWhisperCaptured(out, true)
}
