package components

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
)

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63"))
	labelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	valueStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	warnStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	errStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	okStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	recStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("203")).Background(lipgloss.Color("52"))
)

// StatusPanel renders the main status block.
type StatusPanel struct {
	AppState      string
	Platform      string
	Backend       string
	Provider      string
	SystemDevice  string
	MicDevice     string
	Recording     bool
	Duration      time.Duration
	SessionDir    string
	AutoRecord    bool
	AwaitingConfirm bool
	ConfirmPrompt string
	StatusNote    string
	DetectionWarn string
	ErrorMsg      string
}

func (p StatusPanel) View(width int) string {
	var lines []string
	lines = append(lines, titleStyle.Render("meetctl"))

	if p.Recording && p.AppState == "recording" {
		lines = append(lines, recStyle.Render(" ● RECORDING "))
	}

	lines = append(lines, row("State", p.AppState))
	lines = append(lines, row("Platform", p.Platform))
	lines = append(lines, row("Audio backend", p.Backend))
	if p.SystemDevice != "" {
		lines = append(lines, row("System audio", p.SystemDevice))
	}
	if p.MicDevice != "" {
		lines = append(lines, row("Microphone", p.MicDevice))
	}
	lines = append(lines, row("Meeting", p.Provider))
	if p.Recording {
		lines = append(lines, row("Duration", p.Duration.Round(time.Second).String()))
		lines = append(lines, row("Output", p.SessionDir))
	}
	lines = append(lines, row("Auto-record", fmt.Sprintf("%v", p.AutoRecord)))

	if p.AwaitingConfirm {
		lines = append(lines, warnStyle.Render("⚠ "+p.ConfirmPrompt))
	}
	if p.StatusNote != "" {
		lines = append(lines, okStyle.Render("✓ "+p.StatusNote))
	}

	if p.DetectionWarn != "" {
		lines = append(lines, warnStyle.Render("⚠ "+p.DetectionWarn))
	}
	if p.ErrorMsg != "" {
		lines = append(lines, errStyle.Render("✗ "+p.ErrorMsg))
	}

	body := strings.Join(lines, "\n")
	if width > 0 {
		return lipgloss.NewStyle().Width(width).Render(body)
	}
	return body
}

func row(label, value string) string {
	return labelStyle.Render(label+":") + " " + valueStyle.Render(value)
}

// HelpBar renders keyboard shortcuts.
func HelpBar(awaitingConfirm bool) string {
	if awaitingConfirm {
		return labelStyle.Render("y start recording  ·  n dismiss  ·  q quit")
	}
	keys := []string{
		"q quit", "r record", "a auto-record", "o audio", "d doctor", "s sessions",
	}
	return labelStyle.Render(strings.Join(keys, "  ·  "))
}

// DoctorPanel renders doctor check results.
func DoctorPanel(lines []string) string {
	return titleStyle.Render("Doctor") + "\n" + strings.Join(lines, "\n")
}

// SessionsPanel renders a simple session list.
func SessionsPanel(lines []string) string {
	if len(lines) == 0 {
		return titleStyle.Render("Sessions") + "\n" + labelStyle.Render("(none)")
	}
	return titleStyle.Render("Sessions") + "\n" + strings.Join(lines, "\n")
}

// StatusBadge returns a styled status string.
func StatusBadge(status string) string {
	switch status {
	case "ok":
		return okStyle.Render(status)
	case "warn":
		return warnStyle.Render(status)
	case "fail":
		return errStyle.Render(status)
	default:
		return valueStyle.Render(status)
	}
}
