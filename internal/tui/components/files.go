package components

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"
)

// FileState is where a standalone file stands in the transcription pipeline.
type FileState int

const (
	FilePending FileState = iota
	FileQueued
	FileRunning
	FileDone
	FileFailed
)

// FileRow is one line of the Files list.
type FileRow struct {
	Name     string
	Added    string
	Duration string
	Language string
	State    FileState
	QueuePos int
	Percent  float64
}

// FilesView renders the Files tab: where to drop a file, the files handed over
// so far, and the transcript of the selected one.
//
// Every field is a plain value the Model already holds. Nothing here touches
// the disk or the clock, so View stays a pure function of the Model.
type FilesView struct {
	Width int

	// Rows is the visible window of the list; Cursor indexes into it.
	Rows   []FileRow
	Cursor int
	// First is the position of Rows[0] in the whole list, Total its length.
	First int
	Total int

	Language string // language new files are transcribed with
	Engine   string
	Root     string

	Editing bool
	Input   string

	Note    string
	NoteErr bool

	HasSelection bool
	Selected     FileRow
	SelError     string

	ETA   time.Duration
	Blink bool
	Log   []string

	// Lines is the visible slice of the wrapped transcript, Scroll the index
	// of its first line and TotalLines the wrapped length.
	Lines      []string
	Scroll     int
	TotalLines int
	Words      int
	// TextRows is the height of the transcript body; the panel always renders
	// exactly that many rows so the layout does not jump between states.
	TextRows int
}

// Column widths of the Files list.
const (
	fileColAdded  = 12
	fileColDur    = 8
	fileColLang   = 5
	fileColStatus = 12
	fileColGap    = 2
	fileNameMin   = 12
)

func (v FilesView) View() string {
	width := NewPanelLayout(v.Width).FullWidth()
	return JoinBlocksVertical(v.dropBox(width), v.listBox(width), v.transcriptBox(width))
}

// dropBox is always three rows tall: the invitation (or the path being typed),
// the settings a new file will run with, and the last thing that happened.
func (v FilesView) dropBox(width int) string {
	inner := width - 4
	var first string
	if v.Editing {
		// Keep the tail visible: the end of a long path is the part being typed.
		room := max(inner-len("Path: ")-1, 8)
		first = labelStyle.Render("Path: ") + valueStyle.Render(tailCells(v.Input, room)) + keyStyle.Render("▏")
	} else {
		first = valueStyle.Render("Drag an audio or video file onto this window") +
			subtleStyle.Render(" — or press ") + keyStyle.Render("a") + subtleStyle.Render(" to type a path.")
	}

	lang := v.Language
	if lang == "" {
		lang = "auto-detect"
	}
	settings := labelStyle.Render("language ") + valueStyle.Render(lang) +
		subtleStyle.Render(" (") + keyStyle.Render("l") + subtleStyle.Render(" changes it)")
	if v.Engine != "" {
		settings += subtleStyle.Render(" · ") + labelStyle.Render("engine ") + valueStyle.Render(v.Engine)
	}

	var third string
	switch {
	case v.Note != "" && v.NoteErr:
		third = errStyle.Render(LabelWarn + " " + v.Note)
	case v.Note != "":
		third = okStyle.Render("✓ " + v.Note)
	default:
		third = subtleStyle.Render("Your file is left untouched; the transcript is kept in " + v.Root)
	}

	lines := []string{
		clampStyledWidth(first, inner),
		clampStyledWidth(settings, inner),
		clampStyledWidth(third, inner),
	}
	return Box("Transcribe a file", strings.Join(lines, "\n"), width)
}

func (v FilesView) listBox(width int) string {
	inner := width - 4
	if v.Total == 0 {
		return Box("Files", subtleStyle.Render("Nothing here yet."), width)
	}

	showAdded := inner >= 72
	fixed := fileColDur + fileColLang + fileColStatus + 3*fileColGap
	if showAdded {
		fixed += fileColAdded + fileColGap
	}
	nameW := max(inner-2-fixed, fileNameMin)

	cells := func(name, added, dur, lang, status string) string {
		parts := []string{padCell(name, nameW)}
		if showAdded {
			parts = append(parts, padCell(added, fileColAdded))
		}
		parts = append(parts, padCell(dur, fileColDur), padCell(lang, fileColLang), status)
		return strings.Join(parts, padSpaces(fileColGap))
	}

	lines := []string{subtleStyle.Render("  " + cells("FILE", "ADDED", "LENGTH", "LANG", "STATUS"))}
	for i, r := range v.Rows {
		lang := r.Language
		if lang == "" {
			lang = "auto"
		}
		dur := r.Duration
		if dur == "" {
			dur = "—"
		}
		line := cells(r.Name, r.Added, dur, lang, fileStatusCell(r, v.Blink))
		if i == v.Cursor {
			// Same treatment as the sessions table: strip the row's own colour
			// so the highlight is not cancelled halfway across, and let the
			// marker carry the selection where colour is unavailable.
			line = selRowStyle.Render(padCell("▸ "+ansi.Strip(line), inner))
		} else {
			line = "  " + clampStyledWidth(line, inner-2)
		}
		lines = append(lines, line)
	}

	title := fmt.Sprintf("Files (%d)", v.Total)
	if len(v.Rows) < v.Total {
		title = fmt.Sprintf("Files (%d–%d of %d)", v.First+1, v.First+len(v.Rows), v.Total)
	}
	return Box(title, strings.Join(lines, "\n"), width)
}

func fileStatusCell(r FileRow, blink bool) string {
	switch r.State {
	case FileRunning:
		pulse := "●"
		if blink {
			pulse = "○"
		}
		return txActiveStyle.Render(fmt.Sprintf("%s %d%%", pulse, int(r.Percent)))
	case FileQueued:
		return txActiveStyle.Render(fmt.Sprintf("queue %d", r.QueuePos))
	case FileDone:
		return txDoneStyle.Render("✓ done")
	case FileFailed:
		return txErrorStyle.Render("✗ failed")
	default:
		return txPendingStyle.Render("· not run")
	}
}

func (v FilesView) transcriptBox(width int) string {
	inner := width - 4
	rows := max(v.TextRows, 1)

	var meta string
	var body []string
	switch {
	case !v.HasSelection:
		meta = subtleStyle.Render("No file selected.")
		body = []string{
			"",
			subtleStyle.Render("Drop a voice note, a call recording or a video here and it is transcribed"),
			subtleStyle.Render("with the same engine as your meetings. Press ") + keyStyle.Render("c") +
				subtleStyle.Render(" to copy the text when it is done."),
		}
	case v.Selected.State == FileRunning:
		meta = TranscribeProgressBar(v.Selected.Percent, 24, v.ETA, v.Blink) +
			subtleStyle.Render("  "+v.Selected.Name)
		if len(v.Log) == 0 {
			body = []string{subtleStyle.Render("Starting…")}
		} else {
			// The newest lines are the ones worth watching.
			log := v.Log
			if len(log) > rows {
				log = log[len(log)-rows:]
			}
			for _, l := range log {
				body = append(body, clampStyledWidth(valueStyle.Render(l), inner))
			}
		}
	case v.Selected.State == FileQueued:
		meta = txActiveStyle.Render(fmt.Sprintf("Queued — position %d", v.Selected.QueuePos)) +
			subtleStyle.Render("  "+v.Selected.Name)
		body = []string{subtleStyle.Render("It starts as soon as the transcription ahead of it finishes.")}
	case v.Selected.State == FileFailed:
		meta = txErrorStyle.Render("✗ Could not transcribe") + subtleStyle.Render("  "+v.Selected.Name)
		for line := range strings.SplitSeq(ansi.Wrap(v.SelError, inner, ""), "\n") {
			body = append(body, errStyle.Render(line))
		}
		body = append(body, "", subtleStyle.Render("Press ")+keyStyle.Render("t")+subtleStyle.Render(" to try again."))
	case v.Selected.State == FilePending:
		meta = subtleStyle.Render("Not transcribed yet  " + v.Selected.Name)
		body = []string{subtleStyle.Render("Press ") + keyStyle.Render("t") + subtleStyle.Render(" to transcribe it.")}
	default:
		meta = v.transcriptMeta(inner)
		if v.TotalLines == 0 {
			body = []string{subtleStyle.Render("(the transcript is empty — no speech was recognised)")}
		}
		for _, l := range v.Lines {
			body = append(body, valueStyle.Render(l))
		}
	}

	if len(body) > rows {
		body = body[:rows]
	}
	for len(body) < rows {
		body = append(body, "")
	}
	content := clampStyledWidth(meta, inner) + "\n" + strings.Join(body, "\n")
	return Box("Transcript", content, width)
}

// transcriptMeta is the line above a finished transcript: what it is, and how
// far down the reader has scrolled.
func (v FilesView) transcriptMeta(inner int) string {
	left := valueStyle.Render(v.Selected.Name)
	if v.Selected.Duration != "" {
		left += subtleStyle.Render(" · " + v.Selected.Duration)
	}
	left += subtleStyle.Render(fmt.Sprintf(" · %d words", v.Words))

	right := ""
	if v.TotalLines > len(v.Lines) && len(v.Lines) > 0 {
		right = subtleStyle.Render(fmt.Sprintf("lines %d–%d of %d", v.Scroll+1, v.Scroll+len(v.Lines), v.TotalLines))
	}
	if right == "" {
		return left
	}
	gap := inner - displayWidth(left) - displayWidth(right)
	if gap < 2 {
		return clampStyledWidth(left, max(inner-displayWidth(right)-2, 0)) + "  " + right
	}
	return left + padSpaces(gap) + right
}

// WrapText re-flows text to width cells, keeping its line breaks. It is
// exported because the TUI wraps a transcript once, when the text or the
// terminal changes, and then only slices the result per frame.
func WrapText(text string, width int) []string {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	if width < 8 {
		width = 8
	}
	return strings.Split(ansi.Wrap(text, width, ""), "\n")
}

// tailCells keeps the last max cells of s, marking the cut.
func tailCells(s string, max int) string {
	if displayWidth(s) <= max {
		return s
	}
	return ansi.TruncateLeft(s, displayWidth(s)-max+1, "…")
}

// FilesFooterMode selects footer shortcuts on the Files screen.
type FilesFooterMode int

const (
	FilesFooterNormal FilesFooterMode = iota
	FilesFooterEmpty
	FilesFooterTranscribing
	FilesFooterEditing
)

// FilesFooter returns the Files tab shortcuts, most useful first: FitFooter
// drops from the right when the terminal is narrow.
func FilesFooter(mode FilesFooterMode, width int) string {
	switch mode {
	case FilesFooterEditing:
		return FitFooter(width,
			FooterHint("Enter", "transcribe"),
			FooterHint("Esc", "cancel"),
			FooterHint("Ctrl+C", "quit"),
		)
	case FilesFooterEmpty:
		return FitFooter(width,
			FooterHint("a", "type a path"),
			FooterHint("l", "language"),
			FooterHint("R", "refresh"),
			FooterHint("q", "quit"),
		)
	case FilesFooterTranscribing:
		return FitFooter(width,
			FooterHint("s", "stop transcribe"),
			FooterHint("c", "copy text"),
			FooterHint("↑↓", "select"),
			FooterHint("PgUp/PgDn", "scroll"),
			FooterHint("a", "add path"),
			FooterHint("l", "language"),
			FooterHint("o", "open folder"),
			FooterHint("q", "quit…"),
		)
	default:
		return FitFooter(width,
			FooterHint("c", "copy text"),
			FooterHint("↑↓", "select"),
			FooterHint("PgUp/PgDn", "scroll"),
			FooterHint("a", "add path"),
			FooterHint("t", "transcribe again"),
			FooterHint("l", "language"),
			FooterHint("o", "open folder"),
			FooterHint("d", "remove"),
			FooterHint("q", "quit"),
		)
	}
}
