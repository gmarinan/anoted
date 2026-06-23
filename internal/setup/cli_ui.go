package setup

import (
	"fmt"
	"io"
	"strings"
)

const cliWidth = 52

func printBanner(out io.Writer) {
	fmt.Fprintln(out)
	fmt.Fprintln(out, boxTop())
	fmt.Fprintln(out, boxLine(centerText("anoted setup", cliWidth)))
	fmt.Fprintln(out, boxLine(centerText("first-time configuration", cliWidth)))
	fmt.Fprintln(out, boxBottom())
	fmt.Fprintln(out)
}

func printStepHeader(out io.Writer, step, total int, title string) {
	fmt.Fprintf(out, "  ┌─ Step %d/%d · %s\n", step, total, title)
	fmt.Fprintln(out, "  │")
}

func printStepEnd(out io.Writer) {
	fmt.Fprintln(out, "  └─────────────────────────────────────────────────")
	fmt.Fprintln(out)
}

func printOK(out io.Writer, msg string) {
	fmt.Fprintf(out, "  │  ✓ %s\n", msg)
}

func printWarn(out io.Writer, msg string) {
	fmt.Fprintf(out, "  │  ! %s\n", msg)
}

func printInfo(out io.Writer, msg string) {
	fmt.Fprintf(out, "  │    %s\n", msg)
}

func printChoiceList(out io.Writer, choices []DetectionChoice, selected int) {
	for i, c := range choices {
		marker := " "
		if i == selected {
			marker = "›"
		}
		line := fmt.Sprintf("%s %d) %s", marker, i+1, c.Label)
		if c.Recommended {
			line += " (recommended)"
		}
		fmt.Fprintf(out, "  │  %s\n", line)
		fmt.Fprintf(out, "  │      %s\n", c.Description)
	}
}

func boxTop() string {
	return "  ╭" + strings.Repeat("─", cliWidth) + "╮"
}

func boxBottom() string {
	return "  ╰" + strings.Repeat("─", cliWidth) + "╯"
}

func boxLine(inner string) string {
	const pad = 2
	width := cliWidth - pad
	if len(inner) > width {
		inner = inner[:width]
	}
	return "  │ " + inner + strings.Repeat(" ", width-len(inner)) + " │"
}

func centerText(s string, w int) string {
	if len(s) >= w {
		return s[:w]
	}
	pad := (w - len(s)) / 2
	return strings.Repeat(" ", pad) + s
}
