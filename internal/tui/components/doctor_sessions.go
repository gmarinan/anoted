package components

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"meetctl/internal/doctor"
	"meetctl/internal/session"
	"meetctl/internal/transcribe"
)

// DoctorView renders the Doctor tab.
type DoctorView struct {
	Report        doctor.Report
	AppState      string
	Platform      string
	Backend       string
	Provider      string
	SystemDevice  string
	MicDevice     string
	DetectionWarn string
	Width         int
}

func (v DoctorView) View() string {
	var b strings.Builder

	colW := v.columnWidth()
	summary := v.summaryBox(colW)
	env := v.environmentBox(colW)
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, summary, " ", env))
	b.WriteString("\n\n")
	b.WriteString(v.warningsBox(v.Width))
	return b.String()
}

func (v DoctorView) columnWidth() int {
	w := v.Width
	if w < 60 {
		return 36
	}
	return (w - 3) / 2
}

func (v DoctorView) summaryBox(width int) string {
	var lines []string
	okCount := 0
	for _, c := range v.Report.Checks {
		icon := "✓"
		style := okStyle
		switch c.Status {
		case "warn":
			icon = "!"
			style = warnStyle
		case "fail":
			icon = "✗"
			style = errStyle
		default:
			okCount++
		}
		lines = append(lines, style.Render(fmt.Sprintf("%s %-22s %s", icon, c.Name, c.Detail)))
	}
	header := fmt.Sprintf("%s / %d checks OK", Badge("OK", "ok"), okCount)
	body := header + "\n" + strings.Join(lines, "\n")
	return Box("Diagnostic summary", body, width)
}

func (v DoctorView) environmentBox(width int) string {
	lines := []string{
		row("State", displayState(v.AppState)),
		row("Platform", v.Platform),
		row("Backend", v.Backend),
		row("Meeting", v.Provider),
		row("System audio", truncate(v.SystemDevice, width-14)),
		row("Microphone", truncate(v.MicDevice, width-14)),
	}
	return Box("Environment", strings.Join(lines, "\n"), width)
}

func (v DoctorView) warningsBox(width int) string {
	var warns []string
	for _, c := range v.Report.Checks {
		if c.Status == "warn" || c.Status == "fail" {
			warns = append(warns, fmt.Sprintf("• %s: %s", c.Name, c.Detail))
		}
	}
	if v.DetectionWarn != "" {
		warns = append(warns, "• "+v.DetectionWarn)
	}
	if len(warns) == 0 {
		warns = append(warns, subtleStyle.Render("No warnings — all systems ready."))
	}
	return Box("Warnings / recommendations", strings.Join(warns, "\n"), width)
}

// FolderOpenerChoice is one row in the folder-opener picker.
type FolderOpenerChoice struct {
	ID          string
	Label       string
	Description string
	Available   bool
}

// SessionsView renders the Sessions tab.
type SessionsView struct {
	Records         []session.Record
	Cursor          int
	ErrMsg          string
	Transcribing    bool
	StatusNote      string
	DesktopNote     string
	Width           int
	OpenerPicker    bool
	OpenerCursor    int
	OpenerChoices   []FolderOpenerChoice
	CurrentOpener   string
	OpenerDetected  string
}

func (v SessionsView) View() string {
	var b strings.Builder

	tableW := v.Width
	if tableW < 40 {
		tableW = 80
	}
	b.WriteString(v.tableBox(tableW))
	if v.StatusNote != "" {
		b.WriteString("\n")
		if v.Transcribing {
			b.WriteString(warnStyle.Render("⏳ " + v.StatusNote))
		} else {
			b.WriteString(okStyle.Render("✓ " + v.StatusNote))
		}
	}
	if v.DesktopNote != "" {
		b.WriteString("\n")
		b.WriteString(okStyle.Render("✓ " + v.DesktopNote))
	}
	b.WriteString("\n\n")

	if v.OpenerPicker {
		b.WriteString(v.openerBox(tableW))
		return b.String()
	}

	colW := v.columnWidth()
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, v.detailsBox(colW), " ", v.actionsBox(colW)))
	return b.String()
}

func (v SessionsView) columnWidth() int {
	w := v.Width
	if w < 60 {
		return 36
	}
	return (w - 3) / 2
}

func (v SessionsView) tableBox(width int) string {
	if v.ErrMsg != "" {
		return Box("Sessions", errStyle.Render(v.ErrMsg), width)
	}
	if len(v.Records) == 0 {
		return Box("Sessions", subtleStyle.Render("No recordings yet."), width)
	}

	header := fmt.Sprintf("%-4s  %-16s  %-14s  %-8s  %s",
		"ID", "DATE", "MEET", "DUR", "PATH")
	lines := []string{subtleStyle.Render(header)}
	for i, r := range v.Records {
		line := v.formatRow(r)
		if i == v.Cursor {
			line = lipgloss.NewStyle().
				Background(lipgloss.Color("63")).
				Foreground(lipgloss.Color("229")).
				Render(line)
		}
		lines = append(lines, line)
	}
	return Box("Sessions", strings.Join(lines, "\n"), width)
}

func (v SessionsView) formatRow(r session.Record) string {
	dur := r.Metadata.Duration
	if dur == "" && !r.EndedAt.IsZero() {
		dur = r.EndedAt.Sub(r.StartedAt).Round(time.Second).String()
	}
	if dur == "" {
		dur = "—"
	}
	meet := formatProvider(string(r.Provider))
	if transcribe.HasTranscript(r.Dir) {
		meet = truncate(meet, 10) + " " + Badge("TX", "ok")
	}
	path := filepath.Join(r.Dir, sessionAudioName)
	return fmt.Sprintf("#%-3d  %-16s  %-14s  %-8s  %s",
		r.ID,
		session.FormatLocalTime(r.StartedAt, "2006-01-02 15:04"),
		truncate(meet, 14),
		truncate(dur, 8),
		truncate(path, 40),
	)
}

func (v SessionsView) detailsBox(width int) string {
	if len(v.Records) == 0 {
		return Box("Details", subtleStyle.Render("Select a session"), width)
	}
	if v.Cursor < 0 || v.Cursor >= len(v.Records) {
		return Box("Details", "", width)
	}
	r := v.Records[v.Cursor]
	lines := []string{
		row("ID", fmt.Sprintf("#%d", r.ID)),
		row("Started", session.FormatLocalTime(r.StartedAt, "2006-01-02 15:04:05")),
		row("Provider", formatProvider(string(r.Provider))),
		row("Backend", r.Backend),
		row("Status", string(r.Status)),
		row("Path", truncate(r.Dir, width-8)),
		row("File", sessionAudioName),
	}
	if r.Metadata.Duration != "" {
		lines = append(lines, row("Duration", r.Metadata.Duration))
	}
	if files := transcribe.ListTranscriptFiles(r.Dir); len(files) > 0 {
		lines = append(lines, row("Transcript", truncate(filepath.Base(files[0]), width-12)))
	}
	return Box("Details", strings.Join(lines, "\n"), width)
}

func (v SessionsView) actionsBox(width int) string {
	lines := []string{
		row("Open folders", v.OpenerDetected),
		row("Setting", openerSettingLabel(v.CurrentOpener)),
		"",
		FooterHint("↑↓", "Navigate list"),
		FooterHint("t", "Transcribe (Whisper)"),
		FooterHint("o", "Open session folder"),
		FooterHint("f", "Choose folder opener"),
		FooterHint("p", "Play recording"),
		FooterHint("R", "Refresh list"),
	}
	return Box("Actions (keyboard)", strings.Join(lines, "\n"), width)
}

func (v SessionsView) openerBox(width int) string {
	var lines []string
	lines = append(lines, row("Active", openerSettingLabel(v.CurrentOpener)))
	lines = append(lines, row("Resolves to", truncate(v.OpenerDetected, width-14)))
	lines = append(lines, "")

	for i, opt := range v.OpenerChoices {
		marker := "  "
		if i == v.OpenerCursor {
			marker = "> "
		}
		sel := "○"
		if opt.ID == v.CurrentOpener {
			sel = "●"
		}
		line := fmt.Sprintf("%s%s %s", marker, sel, opt.Label)
		if !opt.Available {
			line += " " + Badge("N/A", "warn")
		}
		if i == v.OpenerCursor {
			line = valueStyle.Bold(true).Render(line)
		} else {
			line = valueStyle.Render(line)
		}
		lines = append(lines, line)
		if i == v.OpenerCursor && opt.Description != "" {
			lines = append(lines, subtleStyle.Render("      "+truncate(opt.Description, width-8)))
		}
	}
	return Box("Open folder with", strings.Join(lines, "\n"), width)
}

func openerSettingLabel(id string) string {
	switch id {
	case "", "auto":
		return "Auto-detect"
	case "xdg-open":
		return "xdg-open"
	default:
		return id
	}
}

func formatProvider(p string) string {
	switch p {
	case "google_meet":
		return "Google Meet"
	case "teams":
		return "Microsoft Teams"
	case "unknown", "":
		return "Unknown"
	default:
		return p
	}
}

const sessionAudioName = "recording.wav"
