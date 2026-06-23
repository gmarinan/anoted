package setup

import (
	"context"
	"fmt"
	"io"
	"os/exec"

	"anoted/internal/config"
	"anoted/internal/platform"
)

// ApplyDetection configures meeting detection and returns status lines for the UI.
func ApplyDetection(cfg *config.Config, plat platform.Info, mode string) []string {
	cfg.Detection.Mode = mode
	var lines []string

	switch mode {
	case DetNone:
		lines = append(lines, "Meeting detection disabled — manual record with r")
	case DetMic:
		if plat.OS == platform.OSWindows {
			lines = append(lines, "Mic activity detection enabled (audio sessions + window titles)")
		} else {
			lines = append(lines, "PipeWire mic activity detection enabled")
			if _, err := exec.LookPath("pactl"); err != nil {
				lines = append(lines, "⚠ pactl not found — install pipewire-pulse or pulseaudio")
			} else if err := verifyPactl(context.Background()); err != nil {
				lines = append(lines, fmt.Sprintf("⚠ pactl check failed: %v", err))
			} else {
				lines = append(lines, "✓ pactl ready")
			}
		}
	case DetWindow:
		if plat.OS == platform.OSWindows {
			if err := verifyWindowsTitles(context.Background()); err != nil {
				lines = append(lines, fmt.Sprintf("⚠ Window titles check: %v", err))
			} else {
				lines = append(lines, "✓ Window titles readable via PowerShell")
			}
		} else if plat.Session == "wayland" {
			lines = append(lines, "⚠ Wayland limits window titles — consider mic mode")
		}
	case DetBoth:
		if plat.Session == "wayland" {
			lines = append(lines, "⚠ Wayland limits window titles; mic leg works best")
		}
	}
	return lines
}

// ApplyWindowTool configures X11 window tool when needed.
func ApplyWindowTool(cfg *config.Config, plat platform.Info, mode, tool string, in io.Reader, out io.Writer, autoInstall bool) ([]string, error) {
	if mode != DetWindow && mode != DetBoth {
		return nil, nil
	}
	if plat.Session != "x11" {
		return nil, nil
	}
	var lines []string
	t, err := setupWindowTool(in, out, tool, autoInstall)
	if err != nil {
		return lines, err
	}
	if t != ToolNone {
		cfg.Detection.WindowTool = t
		lines = append(lines, fmt.Sprintf("✓ Window tool: %s", t))
	}
	return lines, nil
}
