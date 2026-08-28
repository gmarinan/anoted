package components

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"anoted/internal/config"
	"anoted/internal/doctor"
	"anoted/internal/session"
	"anoted/internal/transcribe"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// DoctorView renders the Doctor tab.
type DoctorView struct {
	Report               doctor.Report
	AppState             string
	Platform             string
	Backend              string
	Provider             string
	SystemDevice         string
	MicDevice            string
	DetectionWarn        string
	Width                int
	Height               int
	WhisperInstallActive bool
	WhisperInstallLog    []string
	WhisperInstallErr    string
	WhisperCanInstall    bool
	GPUInstallActive     bool
	GPUInstallLog        []string
	GPUInstallErr        string
	GPUCanInstall        bool
}

func (v DoctorView) View() string {
	layout := NewPanelLayout(v.Width)
	colW := layout.ColumnWidth()
	var b strings.Builder

	summary := v.summaryBox(colW)
	env := v.environmentBox(colW)
	if v.useSingleColumn() {
		b.WriteString(JoinBlocksVertical(summary, env))
	} else {
		b.WriteString(layout.JoinColumns(summary, env))
	}
	b.WriteString("\n\n")
	b.WriteString(v.warningsBox(layout.FullWidth()))
	if v.WhisperInstallActive || v.WhisperInstallErr != "" || len(v.WhisperInstallLog) > 0 {
		b.WriteString("\n\n")
		b.WriteString(v.whisperInstallBox(layout.FullWidth()))
	}
	if v.GPUInstallActive || v.GPUInstallErr != "" || len(v.GPUInstallLog) > 0 {
		b.WriteString("\n\n")
		b.WriteString(v.gpuInstallBox(layout.FullWidth()))
	}
	return b.String()
}

func (v DoctorView) useSingleColumn() bool {
	if v.Height > 0 && v.Height < 30 {
		return true
	}
	return v.Platform == "windows"
}

func (v DoctorView) whisperInstallBox(width int) string {
	var lines []string
	if v.WhisperInstallActive {
		lines = append(lines, warnStyle.Render("Installing Whisper (Python + venv, ~500MB)…"))
	} else if v.WhisperInstallErr != "" {
		lines = append(lines, errStyle.Render("Install failed: "+v.WhisperInstallErr))
	} else if v.WhisperCanInstall {
		lines = append(lines, subtleStyle.Render("Press i to install Whisper without leaving the app."))
	}
	for _, line := range v.WhisperInstallLog {
		lines = append(lines, subtleStyle.Render("  "+truncate(line, width-4)))
	}
	if len(lines) == 0 {
		return ""
	}
	return Box("Whisper setup", strings.Join(lines, "\n"), width)
}

func (v DoctorView) gpuInstallBox(width int) string {
	var lines []string
	if v.GPUInstallActive {
		lines = append(lines, warnStyle.Render("Enabling GPU (PyTorch CUDA, ~1–2 GB)…"))
	} else if v.GPUInstallErr != "" {
		lines = append(lines, errStyle.Render("GPU setup failed: "+v.GPUInstallErr))
	} else if v.GPUCanInstall {
		lines = append(lines, subtleStyle.Render("Press g to enable GPU without leaving the app."))
	}
	for _, line := range v.GPUInstallLog {
		lines = append(lines, subtleStyle.Render("  "+truncate(line, width-4)))
	}
	if len(lines) == 0 {
		return ""
	}
	return Box("GPU setup", strings.Join(lines, "\n"), width)
}

func (v DoctorView) summaryBox(width int) string {
	// The report now arrives asynchronously, so distinguish "not back yet" from
	// a finished run — rendering the usual header here would claim "0 checks OK".
	if len(v.Report.Checks) == 0 {
		return Box("Diagnostic summary", "Running checks…", width)
	}

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
		maxDetail := width - 26
		if maxDetail < 12 {
			maxDetail = 12
		}
		detail := truncate(c.Detail, maxDetail)
		lines = append(lines, style.Render(fmt.Sprintf("%s %-22s %s", icon, c.Name, detail)))
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
			maxDetail := width - len(c.Name) - 6
			if maxDetail < 16 {
				maxDetail = 16
			}
			warns = append(warns, fmt.Sprintf("• %s: %s", c.Name, truncate(c.Detail, maxDetail)))
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

// SessionsView renders the session library (embedded in Home).
type SessionsView struct {
	PageRecords          []session.Record
	Cursor               int
	Page                 int
	PageCount            int
	TotalCount           int
	ErrMsg               string
	DesktopNote          string
	Width                int
	Height               int
	OpenerPicker         bool
	OpenerCursor         int
	OpenerChoices        []FolderOpenerChoice
	CurrentOpener        string
	OpenerDetected       string
	DeleteConfirm        bool
	DeleteCursor         int
	DeleteID             int64
	DeletePath           string
	TranscribeActive     bool
	TranscribeSessionDir string
	TranscribePercent    float64
	TranscribeETA        time.Duration
	TranscribeBlink      bool
	TranscribeLog        []string
	TranscribeErr        string
	TranscribeErrDir     string
	Transcription        config.TranscriptionConfig
	PreviewText          string
}

func (v SessionsView) View() string {
	if v.DeleteConfirm {
		base := v.renderMainContent()
		h := v.overlayHeight()
		return FloatCenter(base, v.renderDeleteModal(), v.Width, h)
	}
	if v.OpenerPicker {
		base := v.renderMainContent()
		h := v.overlayHeight()
		return FloatCenter(base, v.renderOpenerModal(), v.Width, h)
	}
	return v.renderMainContent()
}

func (v SessionsView) overlayHeight() int {
	h := v.Height - 8
	if h < 12 {
		h = 12
	}
	baseH := lipgloss.Height(v.renderMainContent())
	if baseH > h {
		h = baseH
	}
	return h
}

func (v SessionsView) renderMainContent() string {
	layout := NewPanelLayout(v.Width)
	var b strings.Builder

	b.WriteString(v.tableBox(layout.FullWidth()))
	if v.DesktopNote != "" {
		b.WriteString("\n")
		b.WriteString(okStyle.Render("✓ " + v.DesktopNote))
	}
	b.WriteString("\n\n")

	details := v.detailsBox(v.detailsWidth())
	preview := v.previewBox(v.previewWidth())
	if v.detailsPreviewTwoColumn() {
		b.WriteString(layout.JoinColumns(details, preview))
	} else {
		b.WriteString(JoinBlocksVertical(details, preview))
	}
	return b.String()
}

func (v SessionsView) detailsPreviewTwoColumn() bool {
	return v.Width >= HomeTopRowMinWidth
}

func (v SessionsView) previewWidth() int {
	if !v.detailsPreviewTwoColumn() {
		return NewPanelLayout(v.Width).FullWidth()
	}
	detailsW := v.detailsWidth()
	previewW := v.Width - detailsW - panelColumnGap
	if previewW < MinPanelWidth {
		previewW = MinPanelWidth
	}
	return previewW
}

func (v SessionsView) detailsWidth() int {
	if !v.detailsPreviewTwoColumn() {
		return NewPanelLayout(v.Width).FullWidth()
	}
	w := int(float64(v.Width) * 0.42)
	if w < MinPanelWidth {
		w = MinPanelWidth
	}
	max := v.Width - MinPanelWidth - panelColumnGap
	if w > max {
		w = max
	}
	return w
}

func (v SessionsView) previewMode() PreviewMode {
	if v.TranscribeActive {
		rec, ok := v.selectedRecord()
		if ok && rec.Dir == v.TranscribeSessionDir {
			return PreviewTranscribing
		}
	}
	rec, ok := v.selectedRecord()
	if ok && transcribe.HasTranscript(rec.Dir, v.Transcription) {
		return PreviewTranscript
	}
	return PreviewIdle
}

func (v SessionsView) selectedRecord() (session.Record, bool) {
	if v.Cursor < 0 || v.Cursor >= len(v.PageRecords) {
		return session.Record{}, false
	}
	return v.PageRecords[v.Cursor], true
}

func (v SessionsView) previewBox(width int) string {
	return PreviewPanel(v.previewMode(), v.PreviewText, v.TranscribeLog, width)
}

func (v SessionsView) renderOpenerModal() string {
	var lines []string
	lines = append(lines, row("Active", openerSettingLabel(v.CurrentOpener)))
	lines = append(lines, row("Resolves to", truncate(v.OpenerDetected, v.Width-14)))
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
			lines = append(lines, subtleStyle.Render("      "+truncate(opt.Description, v.Width-8)))
		}
	}
	lines = append(lines, "")
	lines = append(lines, subtleStyle.Render("↑↓ choose · Enter apply · Esc cancel"))
	maxW := v.Width - 10
	if maxW < 36 {
		maxW = 36
	}
	if maxW > 64 {
		maxW = 64
	}
	return PickerModal("Open folder with", strings.Join(lines, "\n"), maxW)
}

func (v SessionsView) renderDeleteModal() string {
	var lines []string
	lines = append(lines, warnStyle.Render(fmt.Sprintf("Delete session #%d?", v.DeleteID)))
	lines = append(lines, "")
	lines = append(lines, subtleStyle.Render(truncate(v.DeletePath, v.Width-12)))
	lines = append(lines, "")
	choices := []string{"No", "Yes, delete folder"}
	for i, label := range choices {
		marker := "  "
		if i == v.DeleteCursor {
			marker = "> "
		}
		style := valueStyle
		if i == 0 && v.DeleteCursor == 0 {
			style = valueStyle
		}
		lines = append(lines, marker+style.Render(label))
	}
	lines = append(lines, "")
	lines = append(lines, subtleStyle.Render("↑↓ choose · Enter apply · Esc cancel"))
	maxW := v.Width - 10
	if maxW < 36 {
		maxW = 36
	}
	if maxW > 56 {
		maxW = 56
	}
	return PickerModal("Confirm delete", strings.Join(lines, "\n"), maxW)
}

func (v SessionsView) tableBox(width int) string {
	if v.ErrMsg != "" {
		return Box("Sessions", errStyle.Render(v.ErrMsg), width)
	}
	if v.TotalCount == 0 {
		return Box("Sessions", subtleStyle.Render("No recordings yet."), width)
	}

	title := fmt.Sprintf("Sessions (%d/%d · %d total)", v.Page, v.PageCount, v.TotalCount)
	header := v.tableHeader()
	lines := []string{subtleStyle.Render("  " + header)}
	for i, r := range v.PageRecords {
		line := v.formatRow(r, width)
		if i == v.Cursor {
			// Strip the row's own escapes first: Style.Render only wraps the
			// string, so the first internal reset cancelled the highlight and
			// left the right half of the selected row unhighlighted. The marker
			// carries the selection where colour is unavailable.
			line = clampStyledWidth(ansi.Strip(line), width-6)
			line = lipgloss.NewStyle().
				Background(lipgloss.Color("63")).
				Foreground(lipgloss.Color("229")).
				Render("▸ " + line)
		} else {
			line = "  " + clampStyledWidth(line, width-6)
		}
		lines = append(lines, line)
	}
	return Box(title, strings.Join(lines, "\n"), width)
}

func (v SessionsView) compactTable() bool {
	return v.Width < SessionsCompactWidth
}

func (v SessionsView) ultraCompactTable() bool {
	return v.Width < SessionsUltraCompactWidth
}

func (v SessionsView) tableHeader() string {
	if v.ultraCompactTable() {
		return fmt.Sprintf("%-4s  %-11s  %-8s  %s", "ID", "DATE", "MEET", "TX")
	}
	if v.compactTable() {
		return fmt.Sprintf("%-4s  %-14s  %-9s  %-6s  %s", "ID", "DATE", "MEET", "DUR", "TX")
	}
	return fmt.Sprintf("%-4s  %-16s  %-12s  %-8s  %-28s  %s",
		"ID", "DATE", "MEET", "DUR", "TX", "PATH")
}

func (v SessionsView) formatRow(r session.Record, tableWidth int) string {
	dur := r.Metadata.Duration
	if dur == "" && !r.EndedAt.IsZero() {
		dur = r.EndedAt.Sub(r.StartedAt).Round(time.Second).String()
	}
	if dur == "" {
		dur = "—"
	}
	meet := truncate(formatProvider(string(r.Provider)), meetColWidth(v))
	tx := v.formatTXColumn(r, tableWidth)
	dateFmt := "2006-01-02 15:04"
	if v.ultraCompactTable() {
		dateFmt = "01-02 15:04"
		meet = truncate(meet, 8)
	} else if v.compactTable() {
		dateFmt = "06-01-02 15:04"
	}
	id := fmt.Sprintf("#%-3d", r.ID)
	date := session.FormatLocalTime(r.StartedAt, dateFmt)
	if v.ultraCompactTable() {
		return joinCells(padCell(id, 4), padCell(date, 11), padCell(meet, 8), tx)
	}
	if v.compactTable() {
		return joinCells(padCell(id, 4), padCell(date, 14), padCell(meet, 9),
			padCell(dur, 6), tx)
	}
	path := filepath.Join(r.Dir, sessionAudioName)
	return joinCells(padCell(id, 4), padCell(date, 16), padCell(meet, 12),
		padCell(dur, 8), padCell(tx, 28), truncate(path, 36))
}

// joinCells separates table cells with the same two spaces the header uses.
func joinCells(cells ...string) string {
	return strings.Join(cells, "  ")
}

func meetColWidth(v SessionsView) int {
	switch {
	case v.ultraCompactTable():
		return 8
	case v.compactTable():
		return 10
	default:
		return 12
	}
}

func (v SessionsView) formatTXColumn(r session.Record, tableWidth int) string {
	if v.TranscribeErrDir == r.Dir && v.TranscribeErr != "" {
		return txErrorStyle.Render("err")
	}
	if v.TranscribeActive && r.Dir == v.TranscribeSessionDir {
		if v.compactTable() || tableWidth < 120 {
			return TranscribeProgressCompact(v.TranscribePercent, v.TranscribeBlink)
		}
		barW := 10
		if tableWidth > 100 {
			barW = 14
		}
		return TranscribeProgressBar(v.TranscribePercent, barW, v.TranscribeETA, v.TranscribeBlink)
	}
	if transcribe.HasTranscript(r.Dir, v.Transcription) {
		if v.compactTable() {
			return txDoneStyle.Render("✓")
		}
		return TXStatusLabel("yes")
	}
	if audioExists(r.Dir) {
		if v.compactTable() {
			return txPendingStyle.Render("·")
		}
		return TXStatusLabel("no")
	}
	if v.compactTable() {
		return txPendingStyle.Render("—")
	}
	return TXStatusLabel("no")
}

func audioExists(sessionDir string) bool {
	_, err := os.Stat(filepath.Join(sessionDir, sessionAudioName))
	return err == nil
}

func (v SessionsView) detailsBox(width int) string {
	if v.TotalCount == 0 {
		return Box("Details", subtleStyle.Render("Select a session"), width)
	}
	if v.Cursor < 0 || v.Cursor >= len(v.PageRecords) {
		return Box("Details", "", width)
	}
	r := v.PageRecords[v.Cursor]
	lines := []string{
		row("ID", fmt.Sprintf("#%d", r.ID)),
		row("Started", session.FormatLocalTime(r.StartedAt, "2006-01-02 15:04:05")),
		row("Provider", formatProvider(string(r.Provider))),
		row("Backend", r.Backend),
		row("Status", string(r.Status)),
		row("Path", truncate(r.Dir, width-8)),
		row("File", sessionAudioName),
		row("Open folders", truncate(v.OpenerDetected, width-14)),
		row("Setting", openerSettingLabel(v.CurrentOpener)),
	}
	if r.Metadata.Duration != "" {
		lines = append(lines, row("Duration", r.Metadata.Duration))
	}
	if v.TranscribeActive && r.Dir == v.TranscribeSessionDir {
		barW := 10
		if width > 50 {
			barW = min(14, width-20)
		}
		lines = append(lines, row("TX", TranscribeProgressBar(v.TranscribePercent, barW, v.TranscribeETA, v.TranscribeBlink)))
	}
	return Box("Details", strings.Join(lines, "\n"), width)
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
