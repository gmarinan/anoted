package setup

import (
	"fmt"
	"io"
	"os"
	"strings"

	"anoted/internal/config"
	"anoted/internal/platform"
)

const setupSteps = 4

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

	printBanner(out)

	// Step 1 — config
	printStepHeader(out, 1, setupSteps, "Configuration")
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
	printOK(out, "Config: "+path)
	printStepEnd(out)

	// Step 2 — platform
	printStepHeader(out, 2, setupSteps, "Platform")
	printOK(out, fmt.Sprintf("%s (session: %s)", plat.Name(), plat.Session))
	printStepEnd(out)

	// Step 3 — detection
	printStepHeader(out, 3, setupSteps, "Meeting detection")
	mode := opts.Mode
	if mode == "" {
		mode = chooseDetectionModeInteractive(in, out, plat)
	}
	cfg.Detection.Mode = mode
	for _, line := range ApplyDetection(&cfg, plat, mode) {
		if strings.HasPrefix(line, "⚠") {
			printWarn(out, strings.TrimPrefix(line, "⚠ "))
		} else if strings.HasPrefix(line, "✓") {
			printOK(out, strings.TrimPrefix(line, "✓ "))
		} else {
			printInfo(out, line)
		}
	}
	if lines, err := ApplyWindowTool(&cfg, plat, mode, opts.Tool, in, out, opts.Install); err != nil {
		return cfg, err
	} else {
		for _, line := range lines {
			if strings.HasPrefix(line, "✓") {
				printOK(out, strings.TrimPrefix(line, "✓ "))
			} else {
				printInfo(out, line)
			}
		}
	}
	printStepEnd(out)

	// Step 4 — transcription
	setupTranscription(in, out, &cfg, opts.Install)

	cfg.SetupCompleted = true
	if err := config.Save(path, cfg); err != nil {
		return cfg, err
	}

	fmt.Fprintln(out)
	fmt.Fprintln(out, boxTop())
	fmt.Fprintln(out, boxLine(centerText("Setup complete", cliWidth)))
	fmt.Fprintln(out, boxBottom())
	fmt.Fprintf(out, "  detection.mode: %s\n", cfg.Detection.Mode)
	if cfg.Transcription.AutoAfterRecording {
		fmt.Fprintln(out, "  transcription.auto_after_recording: true")
	}
	fmt.Fprintln(out)
	fmt.Fprintln(out, "  anoted watch   — open the TUI (Setup also available with S)")
	fmt.Fprintln(out, "  anoted doctor  — check dependencies")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "  Tip: setup works while the TUI is open — press R in Config to reload.")
	return cfg, nil
}

func chooseDetectionModeInteractive(in io.Reader, out io.Writer, plat platform.Info) string {
	choices := DetectionChoices(plat)
	fmt.Fprintln(out, "  │  How should anoted detect meetings?")
	fmt.Fprintln(out, "  │")
	for i, c := range choices {
		line := fmt.Sprintf("%d) %s", i+1, c.Label)
		if c.Recommended {
			line += " (recommended)"
		}
		fmt.Fprintf(out, "  │    %s\n", line)
		fmt.Fprintf(out, "  │        %s\n", c.Description)
	}
	fmt.Fprintln(out, "  │")
	defaultChoice := "1"
	for i, c := range choices {
		if c.Recommended {
			defaultChoice = fmt.Sprintf("%d", i+1)
			break
		}
	}
	choice := askChoice(in, out, "  Choice ["+defaultChoice+"]: ", defaultChoice)
	idx := 0
	if n, err := fmt.Sscanf(choice, "%d", &idx); err == nil && n == 1 {
		idx--
		if idx >= 0 && idx < len(choices) {
			return choices[idx].Mode
		}
	}
	return DefaultDetectionMode(plat)
}

// NeedsSetup reports whether the user should run anoted setup.
func NeedsSetup(cfg config.Config, plat platform.Info) bool {
	if cfg.SetupCompleted {
		return false
	}
	return plat.OS == platform.OSLinux || plat.OS == platform.OSWindows || plat.Session == "x11" || plat.Session == "wayland"
}
