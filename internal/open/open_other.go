//go:build !linux

package open

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"anoted/internal/config"
)

// Open launches the desktop handler for path (folder or file).
func Open(path string, cfg config.DesktopConfig, kind Kind) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}
	if _, err := os.Stat(abs); err != nil {
		return fmt.Errorf("path not found: %w", err)
	}

	cmd, args, err := resolveCommand(abs, cfg, kind)
	if err != nil {
		return err
	}
	return exec.Command(cmd, args...).Start()
}

// Detected returns a human-readable description of the opener that would be used.
func Detected(cfg config.DesktopConfig, kind Kind) string {
	cmd, args, err := resolveCommand("/tmp", cfg, kind)
	if err != nil {
		return err.Error()
	}
	if len(args) == 0 {
		return cmd
	}
	return strings.Join(append([]string{cmd}, args[:len(args)-1]...), " ") + " <path>"
}

func resolveCommand(path string, cfg config.DesktopConfig, kind Kind) (string, []string, error) {
	opener := strings.ToLower(strings.TrimSpace(cfg.Opener))
	if opener == "" {
		opener = "auto"
	}

	if kind == KindFile {
		fileOpener := strings.ToLower(strings.TrimSpace(cfg.FileOpener))
		if fileOpener == "" {
			fileOpener = "xdg-open"
		}
		return commandFor(fileOpener, cfg.OpenCommand, path)
	}

	switch opener {
	case "custom":
		return commandFor("custom", cfg.OpenCommand, path)
	case "auto":
		if bin, err := exec.LookPath("xdg-open"); err == nil {
			return bin, []string{path}, nil
		}
		return "", nil, fmt.Errorf("no folder opener found")
	default:
		return commandFor(opener, cfg.OpenCommand, path)
	}
}

func commandFor(opener string, custom []string, path string) (string, []string, error) {
	if opener == "custom" {
		if len(custom) == 0 {
			return "", nil, fmt.Errorf("desktop.opener is custom but desktop.open_command is empty")
		}
		return expandCustomCommand(custom, path)
	}
	if bin, err := exec.LookPath(opener); err == nil {
		return bin, []string{path}, nil
	}
	return "", nil, fmt.Errorf("opener %q not found in PATH", opener)
}

func expandCustomCommand(parts []string, path string) (string, []string, error) {
	if len(parts) == 0 {
		return "", nil, fmt.Errorf("empty open_command")
	}
	args := make([]string, 0, len(parts))
	for _, p := range parts[1:] {
		args = append(args, strings.ReplaceAll(p, "{path}", path))
	}
	return parts[0], args, nil
}

// AvailableOpeners lists configured and auto-detected folder openers.
func AvailableOpeners() []string {
	return []string{"auto", "xdg-open", "custom"}
}
