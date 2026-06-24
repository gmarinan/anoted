//go:build linux

package autostart

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const desktopName = "anoted.desktop"

func available() bool {
	_, err := autostartDir()
	return err == nil
}

func enabled() bool {
	path, err := entryPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

func entryPath() (string, error) {
	dir, err := autostartDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, desktopName), nil
}

func autostartDir() (string, error) {
	cfg, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}
	return filepath.Join(cfg, "autostart"), nil
}

func enable(entry Entry) error {
	dir, err := autostartDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create autostart dir: %w", err)
	}
	path := filepath.Join(dir, desktopName)
	content := renderDesktop(entry)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write autostart entry: %w", err)
	}
	return nil
}

func disable() error {
	path, err := entryPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove autostart entry: %w", err)
	}
	return nil
}

func renderDesktop(entry Entry) string {
	execLine := desktopExecLine(entry.Exec, entry.Args)
	useTerminal := true
	wmClass := entry.WMClass
	if wmClass == "" {
		wmClass = "anoted"
	}
	if len(entry.TerminalCommand) > 0 {
		parts := append(append([]string{}, entry.TerminalCommand...), quoteDesktopField(entry.Exec))
		for _, arg := range entry.Args {
			parts = append(parts, quoteDesktopField(arg))
		}
		execLine = strings.Join(parts, " ")
		useTerminal = false
	}
	var b strings.Builder
	b.WriteString("[Desktop Entry]\n")
	b.WriteString("Type=Application\n")
	b.WriteString("Name=anoted\n")
	b.WriteString("Comment=Meeting detection and local audio recording\n")
	b.WriteString("Exec=" + execLine + "\n")
	if useTerminal {
		b.WriteString("Terminal=true\n")
	} else {
		b.WriteString("Terminal=false\n")
		b.WriteString("StartupWMClass=" + wmClass + "\n")
	}
	b.WriteString("Categories=Utility;AudioVideo;\n")
	b.WriteString("X-GNOME-Autostart-enabled=true\n")
	b.WriteString("StartupNotify=false\n")
	return b.String()
}

func desktopExecLine(exe string, args []string) string {
	parts := append([]string{quoteDesktopField(exe)}, args...)
	return strings.Join(parts, " ")
}

func quoteDesktopField(s string) string {
	if strings.ContainsAny(s, " \t\"\\") {
		return strconvQuote(s)
	}
	return s
}

func strconvQuote(s string) string {
	if !strings.Contains(s, "\"") {
		return `"` + s + `"`
	}
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}
