//go:build linux

package setup

import (
	"fmt"
	"io"
	"os"
	"os/exec"

	"anoted/internal/transcribe"
)

func installTranscription(in io.Reader, out io.Writer, autoInstall bool) error {
	if _, err := transcribe.FindPython(); err != nil {
		if autoInstall && hasCmd("pacman") {
			fmt.Fprintln(out, "  Python missing — installing via pacman…")
			cmd := exec.Command("sudo", "pacman", "-S", "--needed", "--noconfirm", "python")
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("install python: %w", err)
			}
		} else {
			return err
		}
	}
	return transcribe.EnsureWhisperCaptured(out, autoInstall)
}
