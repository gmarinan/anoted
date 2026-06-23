package setup

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"anoted/internal/platform"
)

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

// chooseDetectionMode is kept for tests.
func chooseDetectionMode(in io.Reader, out io.Writer, plat platform.Info) string {
	return chooseDetectionModeInteractive(in, out, plat)
}
