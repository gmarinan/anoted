//go:build !linux

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
	return []OpenerOption{
		{ID: "auto", Label: "Auto-detect", Description: "System default", Available: true},
		{ID: "xdg-open", Label: "xdg-open", Description: "System handler", Available: true},
	}
}

// CurrentOpenerID returns the active opener id (default auto).
func CurrentOpenerID(cfg config.DesktopConfig) string {
	if cfg.Opener != "" {
		return cfg.Opener
	}
	return "auto"
}
