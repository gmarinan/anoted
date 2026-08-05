//go:build linux

package open

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

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
	c := exec.Command(cmd, args...)
	return c.Start()
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
		return autoFolderCommand(path)
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

	bin, args, ok := knownOpener(opener, path)
	if !ok {
		return "", nil, fmt.Errorf("unknown opener %q", opener)
	}
	return bin, args, nil
}

func autoFolderCommand(path string) (string, []string, error) {
	for _, name := range folderManagerOrder {
		if bin, err := exec.LookPath(name); err == nil {
			return bin, []string{path}, nil
		}
	}
	if desktop := xdgFolderDesktop(); desktop != "" && !isDiskUsageHandler(desktop) {
		if bin, err := exec.LookPath("xdg-open"); err == nil {
			return bin, []string{path}, nil
		}
	}
	if bin, err := exec.LookPath("xdg-open"); err == nil {
		return bin, []string{path}, nil
	}
	return "", nil, fmt.Errorf("no folder opener found — press f in Sessions to choose one")
}

var folderManagerOrder = []string{
	"dolphin",
	"nautilus",
	"thunar",
	"pcmanfm",
	"nemo",
	"caja",
}

func knownOpener(name, path string) (string, []string, bool) {
	switch name {
	case "xdg-open":
		if bin, err := exec.LookPath("xdg-open"); err == nil {
			return bin, []string{path}, true
		}
	case "explorer":
		if bin, err := exec.LookPath("explorer"); err == nil {
			return bin, []string{path}, true
		}
	default:
		if bin, err := exec.LookPath(name); err == nil {
			return bin, []string{path}, true
		}
	}
	return "", nil, false
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

var xdgFolderDesktopOnce struct {
	sync.Once
	value string
}

// xdgFolderDesktop reports the desktop file registered for inode/directory.
//
// The result is cached for the process lifetime: this spawns xdg-mime, and it
// used to be reached from View(), which Bubble Tea calls after every message —
// roughly 30 times a second while the level meter ticks. The user's default
// file manager does not change mid-session.
func xdgFolderDesktop() string {
	xdgFolderDesktopOnce.Do(func() {
		out, err := exec.Command("xdg-mime", "query", "default", "inode/directory").Output()
		if err != nil {
			return
		}
		xdgFolderDesktopOnce.value = strings.TrimSpace(string(out))
	})
	return xdgFolderDesktopOnce.value
}

func isDiskUsageHandler(desktop string) bool {
	lower := strings.ToLower(desktop)
	for _, needle := range []string{
		"baobab", "filelight", "dirstat", "qdirstat", "gdmap", "ncdu",
	} {
		if strings.Contains(lower, needle) {
			return true
		}
	}
	return false
}

// AvailableOpeners lists configured and auto-detected folder openers for docs/UI.
func AvailableOpeners() []string {
	out := []string{"auto", "xdg-open", "custom"}
	for _, name := range folderManagerOrder {
		if _, err := exec.LookPath(name); err == nil {
			out = append(out, name)
		}
	}
	return out
}
