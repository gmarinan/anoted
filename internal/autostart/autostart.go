package autostart

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrUnavailable means login autostart is not supported on this host.
var ErrUnavailable = errors.New("login autostart is not available on this platform")

// Entry describes how the app is launched at login.
type Entry struct {
	Exec            string
	Args            []string
	WMClass         string   // StartupWMClass when using a terminal wrapper
	TerminalCommand []string // e.g. alacritty --class anoted -e
}

// DefaultEntry returns the standard launch command for the current binary.
func DefaultEntry() (Entry, error) {
	exe, err := ResolveExec()
	if err != nil {
		return Entry{}, err
	}
	return Entry{Exec: exe, Args: []string{"watch"}}, nil
}

// ResolveExec returns the absolute path to the running anoted binary.
func ResolveExec() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable: %w", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return "", fmt.Errorf("resolve executable symlink: %w", err)
	}
	return filepath.Abs(exe)
}

// Available reports whether login autostart can be configured on this host.
func Available() bool {
	return available()
}

// Enabled reports whether login autostart is currently installed.
func Enabled() bool {
	return enabled()
}

// Path returns the autostart entry path when supported.
func Path() (string, error) {
	return entryPath()
}

// Enable installs login autostart for entry.
func Enable(entry Entry) error {
	if entry.Exec == "" {
		var err error
		entry, err = DefaultEntry()
		if err != nil {
			return err
		}
	}
	if entry.WMClass == "" {
		entry.WMClass = "anoted"
	}
	return enable(entry)
}

// Disable removes login autostart.
func Disable() error {
	return disable()
}
