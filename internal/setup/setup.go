package setup

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"anoted/internal/config"
	"anoted/internal/platform"
)

const (
	ToolAuto    = "auto"
	ToolXdotool = "xdotool"
	ToolWmctrl  = "wmctrl"
	ToolNone    = "none"

	DetMic    = "mic"
	DetWindow = "window"
	DetBoth   = "both"
	DetNone   = "none"
)

// Options configures the guided setup flow.
type Options struct {
	Reader  io.Reader
	Writer  io.Writer
	Mode    string // mic, window, both, none
	Tool    string // xdotool, wmctrl, none (window/both on X11)
	Install bool
}

// Run performs interactive first-time setup.
func Run(cfg config.Config, cfgPath string, plat platform.Info, opts Options) (config.Config, error) {
	in := opts.Reader
	if in == nil {
		in = os.Stdin
	}
	out := opts.Writer
	if out == nil {
		out = os.Stdout
	}

	fmt.Fprintln(out, "anoted setup")
	fmt.Fprintln(out, "─────────────")
	fmt.Fprintln(out)

	fmt.Fprintln(out, "[1/4] Configuration")
	path, err := config.EnsureDefault()
	if err != nil {
		return cfg, err
	}
	if cfgPath != "" {
		path = cfgPath
	}
	cfg, err = config.Load(path)
	if err != nil {
		return cfg, err
	}
	fmt.Fprintf(out, "  ✓ Config: %s\n\n", path)

	fmt.Fprintln(out, "[2/4] Platform")
	fmt.Fprintf(out, "  ✓ %s (session: %s)\n\n", plat.Name(), plat.Session)

	fmt.Fprintln(out, "[3/4] Meeting detection")
	mode := opts.Mode
	if mode == "" {
		mode = chooseDetectionMode(in, out, plat)
	}
	cfg.Detection.Mode = mode

	switch mode {
	case DetNone:
		fmt.Fprintln(out, "\n  ○ Meeting detection disabled (manual record with r)")
	case DetMic:
		fmt.Fprintln(out, "\n  Using PipeWire mic activity (pactl)")
		if _, err := exec.LookPath("pactl"); err != nil {
			fmt.Fprintln(out, "  ⚠ pactl not found — install pipewire-pulse or pulseaudio")
		} else if err := verifyPactl(context.Background()); err != nil {
			fmt.Fprintf(out, "  ⚠ pactl check failed: %v\n", err)
		} else {
			fmt.Fprintln(out, "  ✓ pactl ready")
		}
	case DetWindow, DetBoth:
		if plat.Session == "wayland" {
			fmt.Fprintln(out, "  Wayland limits window titles; mic mode works better.")
		}
		tool := opts.Tool
		if plat.Session == "x11" {
			tool, err = setupWindowTool(in, out, tool, opts.Install)
			if err != nil {
				return cfg, err
			}
			if tool != ToolNone {
				cfg.Detection.WindowTool = tool
			}
		} else if mode == DetWindow {
			fmt.Fprintln(out, "  ⚠ Window detection unavailable; switching to mic mode")
			cfg.Detection.Mode = DetMic
		}
	}

	setupTranscription(in, out, &cfg, opts.Install)

	cfg.SetupCompleted = true
	if err := config.Save(path, cfg); err != nil {
		return cfg, err
	}

	fmt.Fprintln(out)
	fmt.Fprintln(out, "Setup complete!")
	fmt.Fprintf(out, "  detection.mode: %s\n", cfg.Detection.Mode)
	if cfg.Transcription.AutoAfterRecording {
		fmt.Fprintln(out, "  transcription.auto_after_recording: true")
	}
	fmt.Fprintln(out, "  anoted watch   — open the TUI")
	fmt.Fprintln(out, "  anoted doctor  — check dependencies")
	return cfg, nil
}

func chooseDetectionMode(in io.Reader, out io.Writer, plat platform.Info) string {
	fmt.Fprintln(out, "  How should anoted detect meetings?")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "    1) PipeWire mic  — when Meet/Teams uses your microphone (recommended)")
	fmt.Fprintln(out, "    2) Window titles — browser tab titles via xdotool/wmctrl (X11)")
	fmt.Fprintln(out, "    3) Both          — mic first, then window titles")
	fmt.Fprintln(out, "    4) Skip          — manual recording only")
	fmt.Fprintln(out)
	if plat.Session == "wayland" {
		fmt.Fprintln(out, "  Tip: on Wayland, option 1 works best.")
		fmt.Fprintln(out)
	}
	choice := askChoice(in, out, "  Choice [1]: ", "1")
	switch choice {
	case "2":
		return DetWindow
	case "3":
		return DetBoth
	case "4":
		return DetNone
	default:
		return DetMic
	}
}

func setupWindowTool(in io.Reader, out io.Writer, preset string, autoInstall bool) (string, error) {
	tool, err := chooseWindowTool(in, out, preset, hasTool(ToolXdotool), hasTool(ToolWmctrl))
	if err != nil {
		return "", err
	}
	if tool == ToolNone {
		return tool, nil
	}
	if hasTool(tool) {
		fmt.Fprintf(out, "\n  ✓ %s ready at %s\n", tool, toolPath(tool))
		if err := verifyTitles(context.Background(), tool); err != nil {
			fmt.Fprintf(out, "  ⚠ Could not read window titles: %v\n", err)
		} else {
			fmt.Fprintln(out, "  ✓ Window titles readable")
		}
		return tool, nil
	}
	fmt.Fprintf(out, "\n  %s is not installed.\n", tool)
	if autoInstall || askYes(in, out, "Install it now? (needs sudo) [Y/n]: ") {
		if err := installTool(out, tool); err != nil {
			return tool, err
		}
	} else {
		printManualInstall(out, tool)
	}
	return tool, nil
}

func chooseWindowTool(in io.Reader, out io.Writer, preset string, hasXdotool, hasWmctrl bool) (string, error) {
	if preset != "" {
		return preset, nil
	}
	if hasXdotool {
		fmt.Fprintf(out, "  ✓ xdotool already installed\n")
		if askYes(in, out, "  Use xdotool for window titles? [Y/n]: ") {
			return ToolXdotool, nil
		}
	}
	if hasWmctrl {
		fmt.Fprintf(out, "  ✓ wmctrl already installed\n")
		return ToolWmctrl, nil
	}
	fmt.Fprintln(out, "  Window detection needs xdotool or wmctrl on X11:")
	fmt.Fprintln(out, "    1) xdotool")
	fmt.Fprintln(out, "    2) wmctrl")
	fmt.Fprintln(out, "    3) Skip window tool")
	fmt.Fprintln(out)
	choice := askChoice(in, out, "  Choice [1]: ", "1")
	switch choice {
	case "2":
		return ToolWmctrl, nil
	case "3":
		return ToolNone, nil
	default:
		return ToolXdotool, nil
	}
}

func verifyPactl(ctx context.Context) error {
	_, err := exec.CommandContext(ctx, "pactl", "list", "source-outputs", "short").Output()
	return err
}

func askChoice(in io.Reader, out io.Writer, prompt, defaultVal string) string {
	fmt.Fprint(out, prompt)
	reader := bufio.NewReader(in)
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return defaultVal
	}
	return line
}

func askYes(in io.Reader, out io.Writer, prompt string) bool {
	ans := askChoice(in, out, prompt, "y")
	return ans == "" || strings.EqualFold(ans, "y") || strings.EqualFold(ans, "yes")
}

func askNo(in io.Reader, out io.Writer, prompt string) bool {
	ans := askChoice(in, out, prompt, "n")
	return strings.EqualFold(ans, "y") || strings.EqualFold(ans, "yes")
}

func hasTool(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func toolPath(name string) string {
	p, err := exec.LookPath(name)
	if err != nil {
		return name
	}
	return p
}

func printManualInstall(out io.Writer, tool string) {
	if cmd, ok := installCommand(tool); ok {
		fmt.Fprintf(out, "\n  Install manually:\n    %s\n", strings.Join(cmd, " "))
	} else {
		fmt.Fprintf(out, "\n  Install %s using your package manager.\n", tool)
	}
}

func verifyTitles(ctx context.Context, tool string) error {
	switch tool {
	case ToolXdotool:
		if !hasTool(ToolXdotool) {
			return fmt.Errorf("xdotool not found")
		}
		out, err := exec.CommandContext(ctx, "xdotool", "getactivewindow").Output()
		if err != nil {
			return err
		}
		if strings.TrimSpace(string(out)) == "" {
			return fmt.Errorf("no active window")
		}
	case ToolWmctrl:
		if !hasTool(ToolWmctrl) {
			return fmt.Errorf("wmctrl not found")
		}
		_, err := exec.CommandContext(ctx, "wmctrl", "-l").Output()
		return err
	}
	return nil
}

// NeedsSetup reports whether the user should run anoted setup.
func NeedsSetup(cfg config.Config, plat platform.Info) bool {
	if cfg.SetupCompleted {
		return false
	}
	return plat.OS == platform.OSLinux || plat.Session == "x11" || plat.Session == "wayland"
}
