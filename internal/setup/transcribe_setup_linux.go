//go:build linux

package setup

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"anoted/internal/transcribe"
)

func installTranscription(in io.Reader, out io.Writer, autoInstall bool) error {
	if _, err := transcribe.FindPython(); err != nil {
		switch {
		case in == nil:
			// No interactive terminal. The in-TUI wizard reaches here while
			// Bubble Tea owns the terminal in raw mode: sudo would print its
			// password prompt invisibly over the alternate screen and then block
			// on a stdin that Bubble Tea is already reading, hanging the wizard
			// on "Installing…" with no way out. Send the user to a real shell.
			return fmt.Errorf("python is required and installing it needs root: "+
				"run `anoted setup` in a terminal, or install python yourself (%w)", err)
		case autoInstall && hasCmd("pacman"):
			// Escalating privileges silently is exactly the kind of surprise
			// this project should not spring on anyone; show the command first,
			// the way the whisper installer already does.
			args := []string{"sudo", "pacman", "-S", "--needed", "--noconfirm", "python"}
			fmt.Fprintf(out, "  Python missing — running: %s\n\n", strings.Join(args, " "))
			cmd := exec.Command(args[0], args[1:]...)
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("install python: %w", err)
			}
		default:
			return err
		}
	}
	return transcribe.EnsureWhisperCaptured(out, autoInstall)
}
