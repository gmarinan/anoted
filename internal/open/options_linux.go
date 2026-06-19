//go:build linux

package open

import (
	"os/exec"
	"strings"

	"meetctl/internal/config"
)

// OpenerOption is a selectable folder opener in the TUI.
type OpenerOption struct {
	ID          string
	Label       string
	Description string
	Available   bool
}

// OpenerOptions returns choices for the Sessions folder-opener picker.
func OpenerOptions(cfg config.DesktopConfig) []OpenerOption {
	xdgDefault := xdgFolderDesktop()
	var opts []OpenerOption

	autoDesc := "File manager first, skips disk-usage apps (Baobab…)"
	if xdgDefault != "" {
		if isDiskUsageHandler(xdgDefault) {
			autoDesc += " — xdg default is " + trimDesktop(xdgDefault) + " (skipped)"
		} else {
			autoDesc += " — xdg default: " + trimDesktop(xdgDefault)
		}
	}
	if det := Detected(config.DesktopConfig{Opener: "auto"}, KindFolder); det != "" {
		autoDesc += " → " + det
	}
	opts = append(opts, OpenerOption{
		ID:          "auto",
		Label:       "Auto-detect (recommended)",
		Description: autoDesc,
		Available:   true,
	})

	if _, err := exec.LookPath("xdg-open"); err == nil {
		desc := "Uses system MIME handler for folders"
		if xdgDefault != "" {
			desc += " (" + trimDesktop(xdgDefault) + ")"
		}
		opts = append(opts, OpenerOption{
			ID:          "xdg-open",
			Label:       "xdg-open (system default)",
			Description: desc,
			Available:   true,
		})
	}

	for _, name := range folderManagerOrder {
		bin, err := exec.LookPath(name)
		avail := err == nil
		desc := "Not installed"
		if avail {
			desc = bin
		}
		opts = append(opts, OpenerOption{
			ID:          name,
			Label:       name,
			Description: desc,
			Available:   avail,
		})
	}
	return opts
}

// CurrentOpenerID returns the active opener id (default auto).
func CurrentOpenerID(cfg config.DesktopConfig) string {
	if id := strings.TrimSpace(cfg.Opener); id != "" {
		return id
	}
	return "auto"
}

func trimDesktop(desktop string) string {
	desktop = strings.TrimSuffix(desktop, ".desktop")
	if i := strings.LastIndex(desktop, "."); i >= 0 {
		return desktop[i+1:]
	}
	return desktop
}
