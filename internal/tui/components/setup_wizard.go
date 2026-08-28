package components

import (
	"fmt"
	"strings"

	"anoted/internal/config"
	"anoted/internal/setup"
)

// SetupWizardView renders the floating first-run setup modal.
type SetupWizardView struct {
	State      setup.WizardState
	Choices    []setup.DetectionChoice
	Config     config.Config
	ConfigPath string
	Platform   string
	Width      int
	Height     int
	Summary    []string
	// InstallFrame drives the spinner while dependencies install.
	InstallFrame uint64
}

func (v SetupWizardView) View(base string) string {
	if v.State.Step == setup.WizardInstalling || v.State.Step == setup.WizardDone {
		h := v.overlayHeight()
		return FloatCenter(base, v.renderModal(), v.Width, h)
	}
	h := v.overlayHeight()
	return FloatCenter(base, v.renderModal(), v.Width, h)
}

func (v SetupWizardView) overlayHeight() int {
	h := v.Height - 4
	if h < 16 {
		h = 16
	}
	return h
}

func (v SetupWizardView) renderModal() string {
	var b strings.Builder
	title := fmt.Sprintf("Setup · step %d/4 · %s", v.State.StepNumber(), v.State.StepTitle())
	b.WriteString(boxTitleStyle.Render(strings.ToUpper(title)))
	b.WriteString("\n\n")
	b.WriteString(v.renderBody())
	if v.State.Err != "" {
		b.WriteString("\n")
		b.WriteString(errStyle.Render("✗ " + truncate(v.State.Err, v.Width-8)))
	}
	b.WriteString("\n\n")
	b.WriteString(subtleStyle.Render(v.footerHints()))
	maxW := v.Width - 8
	if maxW < 48 {
		maxW = 48
	}
	if maxW > 72 {
		maxW = 72
	}
	return modalBoxStyle.Width(maxW).Render(b.String())
}

func (v SetupWizardView) renderBody() string {
	switch v.State.Step {
	case setup.WizardWelcome:
		return v.renderWelcome()
	case setup.WizardDetection:
		return v.renderDetection()
	case setup.WizardTranscription:
		return v.renderTranscription()
	case setup.WizardInstalling:
		return v.renderInstalling()
	case setup.WizardDone:
		return v.renderDone()
	default:
		return ""
	}
}

func (v SetupWizardView) renderWelcome() string {
	lines := []string{
		"Welcome to anoted.",
		"",
		"Quick setup for meeting detection and optional Whisper transcription.",
		"",
		subtleStyle.Render("Config: " + truncate(v.ConfigPath, v.Width-12)),
		subtleStyle.Render("Platform: " + v.Platform),
	}
	return strings.Join(lines, "\n")
}

func (v SetupWizardView) renderDetection() string {
	var lines []string
	lines = append(lines, "How should anoted detect meetings?")
	lines = append(lines, "")
	for i, c := range v.Choices {
		marker := "  "
		if i == v.State.DetCursor {
			marker = "> "
		}
		label := fmt.Sprintf("%s%d. %s", marker, i+1, c.Label)
		if c.Recommended {
			label += " " + Badge("recommended", "ok")
		}
		if i == v.State.DetCursor {
			label = valueStyle.Bold(true).Render(label)
		}
		lines = append(lines, label)
		if i == v.State.DetCursor {
			lines = append(lines, subtleStyle.Render("   "+truncate(c.Description, v.Width-8)))
		}
	}
	for _, line := range v.State.DetectionLines {
		if strings.HasPrefix(line, "⚠") {
			lines = append(lines, warnStyle.Render("   "+line))
		} else if strings.HasPrefix(line, "✓") {
			lines = append(lines, okStyle.Render("   "+line))
		} else {
			lines = append(lines, subtleStyle.Render("   "+line))
		}
	}
	return strings.Join(lines, "\n")
}

func (v SetupWizardView) renderTranscription() string {
	opts := []struct {
		label string
		on    bool
	}{
		{"Auto-transcribe after each recording", v.State.AutoTranscribe},
		{"Install Whisper in local venv (~500MB)", v.State.InstallWhisper},
	}
	if setup.TranscribeOptionCount(v.Config) >= 3 {
		opts = append(opts, struct {
			label string
			on    bool
		}{"Enable GPU (CUDA) — ~1–2 GB download", v.State.EnableGPU})
	}
	var lines []string
	lines = append(lines, "Local speech-to-text → txt / srt / md (Obsidian) per session")
	lines = append(lines, "")
	for i, o := range opts {
		marker := "  "
		if i == v.State.TranscribeCursor {
			marker = "> "
		}
		sel := "○"
		if o.on {
			sel = "●"
		}
		line := fmt.Sprintf("%s%s %s", marker, sel, o.label)
		if i == v.State.TranscribeCursor {
			line = valueStyle.Bold(true).Render(line)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func (v SetupWizardView) renderInstalling() string {
	logH := v.logViewHeight()
	lines := []string{
		warnStyle.Render(Spinner(v.InstallFrame) + " Installing dependencies…"),
		"",
		subtleStyle.Render("Install log (PgUp/PgDn to scroll):"),
	}
	visible := v.State.VisibleLog(logH)
	if len(visible) == 0 {
		lines = append(lines, subtleStyle.Render("  (waiting for output…)"))
	} else {
		for _, line := range visible {
			lines = append(lines, subtleStyle.Render("  "+truncate(line, v.Width-12)))
		}
	}
	if len(v.State.Log) > logH {
		lines = append(lines, subtleStyle.Render(fmt.Sprintf("  … %d lines total", len(v.State.Log))))
	}
	return strings.Join(lines, "\n")
}

func (v SetupWizardView) renderDone() string {
	var lines []string
	lines = append(lines, okStyle.Render("Setup complete!"))
	lines = append(lines, "")
	for _, s := range v.Summary {
		lines = append(lines, "  "+s)
	}
	return strings.Join(lines, "\n")
}

func (v SetupWizardView) logViewHeight() int {
	h := v.Height / 3
	if h < 6 {
		h = 6
	}
	if h > 14 {
		h = 14
	}
	return h
}

func (v SetupWizardView) footerHints() string {
	if v.State.Busy {
		return "Installing… please wait"
	}
	switch v.State.Step {
	case setup.WizardWelcome, setup.WizardDone:
		return "Enter continue · Esc defer · q quit"
	case setup.WizardDetection:
		return "↑↓ choose · Enter continue · Esc back · q quit"
	case setup.WizardTranscription:
		return "↑↓ option · Space toggle · Enter continue · Esc back · q quit"
	case setup.WizardInstalling:
		return "PgUp/PgDn scroll log · q quit"
	default:
		return ""
	}
}
