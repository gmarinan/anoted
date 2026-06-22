//go:build windows

package open

import "anoted/internal/config"

// OpenerOption is a selectable folder opener in the TUI.
type OpenerOption struct {
	ID          string
	Label       string
	Description string
	Available   bool
}

// OpenerOptions returns choices for the Sessions folder-opener picker.
func OpenerOptions(cfg config.DesktopConfig) []OpenerOption {
	_, _, err := explorerCommand(`C:\`)
	explorerOK := err == nil
	return []OpenerOption{
		{ID: "auto", Label: "Auto-detect", Description: "explorer.exe", Available: explorerOK},
		{ID: "explorer", Label: "Explorer", Description: "Windows File Explorer", Available: explorerOK},
		{ID: "custom", Label: "Custom command", Description: "desktop.open_command", Available: len(cfg.OpenCommand) > 0},
	}
}

// CurrentOpenerID returns the active opener id (default auto).
func CurrentOpenerID(cfg config.DesktopConfig) string {
	if cfg.Opener != "" {
		return cfg.Opener
	}
	return "auto"
}
