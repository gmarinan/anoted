package components

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// HelpEntry is one row of the keyboard shortcut overlay.
type HelpEntry struct {
	Key    string
	Action string
}

// HelpSection groups shortcuts under a heading.
type HelpSection struct {
	Title   string
	Entries []HelpEntry
}

// HelpView renders the full keyboard map as a centered overlay.
//
// Before this existed the only place shortcuts were listed was the footer,
// which wraps when the terminal is narrow and is pushed off screen when it is
// short — so on a default 80x24 terminal there was no way to discover any
// binding at all.
type HelpView struct {
	Sections []HelpSection
	// Current is the title of the open tab's section. It is listed first and
	// is the last to go when the terminal is too short for all of them.
	Current string
	Width   int
	Height  int
}

const (
	helpKeyColumn = 14
	// helpColumnWidth is the widest a column of shortcuts gets, and
	// helpColumnMin the narrowest before fewer columns are used.
	helpColumnWidth = 58
	helpColumnMin   = 50
	helpColumnGap   = 4
	// helpModalChrome is the modal's border and padding: 2+2 rows, 2+4 cells.
	helpModalChromeRows = 4
	helpModalChromeCols = 6
)

// Overlay floats the shortcut list above the current screen.
//
// It used to be one 52-row column, taller than any common terminal, so the
// frame ran past the bottom and the close hint was never visible. Sections now
// flow into as many columns as the width allows, and the ones for other tabs
// are dropped when even that does not fit.
func (v HelpView) Overlay(base string) string {
	maxW := v.Width - 4
	if v.Width <= 0 {
		maxW = helpColumnWidth*2 + helpColumnGap + helpModalChromeCols
	}
	room := maxW - helpModalChromeCols
	ncols := max(1, min(3, (room+helpColumnGap)/(helpColumnMin+helpColumnGap)))
	colW := max(min(helpColumnWidth, (room-(ncols-1)*helpColumnGap)/ncols), 24)
	// Title row, the blank under the body and the close hint.
	budget := v.Height - helpModalChromeRows - 3
	if v.Height <= 0 {
		budget = 1 << 30
	}

	sections := v.ordered()
	dropped := false
	var cols [][]string
	for {
		blocks := make([][]string, len(sections))
		for i, sec := range sections {
			blocks[i] = helpBlock(sec, colW)
		}
		cols = flowColumns(blocks, ncols)
		if tallest(cols) <= budget || len(sections) <= 1 {
			break
		}
		sections = sections[:len(sections)-1]
		dropped = true
	}

	body := joinHelpColumns(cols, colW)
	if lines := strings.Split(body, "\n"); budget > 0 && len(lines) > budget {
		body = strings.Join(lines[:budget], "\n") // a terminal too small even for one section
	}
	hint := "Press ? or Esc to close"
	if dropped {
		hint = "More keys: open a tab and press ?  ·  ? or Esc closes"
	}
	body += "\n\n" + subtleStyle.Render(clampStyledWidth(hint, ncols*colW+(ncols-1)*helpColumnGap))
	modal := PickerModal("Keyboard shortcuts", body, maxW)
	return FloatCenter(base, modal, v.Width, v.Height)
}

// ordered puts the open tab first, then Global; the rest keep their order.
// The open tab's keys are the ones the user came for, and Global's are
// mostly on screen already (the tab bar and the footer).
func (v HelpView) ordered() []HelpSection {
	var current, global, rest []HelpSection
	for _, sec := range v.Sections {
		switch sec.Title {
		case v.Current:
			current = append(current, sec)
		case "Global":
			global = append(global, sec)
		default:
			rest = append(rest, sec)
		}
	}
	return append(append(current, global...), rest...)
}

// helpBlock renders one section as lines no wider than width, wrapping long
// actions under themselves rather than under the key column.
func helpBlock(sec HelpSection, width int) []string {
	lines := []string{boxTitleStyle.Render(sec.Title)}
	actionW := max(width-2-helpKeyColumn, 10)
	indent := strings.Repeat(" ", 2+helpKeyColumn)
	for _, e := range sec.Entries {
		wrapped := strings.Split(ansi.Wrap(e.Action, actionW, ""), "\n")
		lines = append(lines, "  "+padCell(keyStyle.Render(e.Key), helpKeyColumn)+subtleStyle.Render(wrapped[0]))
		for _, w := range wrapped[1:] {
			lines = append(lines, indent+subtleStyle.Render(w))
		}
	}
	return lines
}

// flowColumns fills columns in order, starting a new one once a column reaches
// its share of the total, so the tallest column stays close to the average.
func flowColumns(blocks [][]string, ncols int) [][]string {
	total := 0
	for _, b := range blocks {
		total += len(b) + 1
	}
	target := (total + ncols - 1) / ncols
	cols := [][]string{nil}
	for _, b := range blocks {
		cur := len(cols) - 1
		if len(cols[cur]) > 0 && len(cols[cur])+len(b) > target && len(cols) < ncols {
			cols = append(cols, nil)
			cur++
		}
		if len(cols[cur]) > 0 {
			cols[cur] = append(cols[cur], "")
		}
		cols[cur] = append(cols[cur], b...)
	}
	return cols
}

func tallest(cols [][]string) int {
	h := 0
	for _, c := range cols {
		h = max(h, len(c))
	}
	return h
}

func joinHelpColumns(cols [][]string, colW int) string {
	h := tallest(cols)
	gap := strings.Repeat(" ", helpColumnGap)
	rows := make([]string, h)
	for r := 0; r < h; r++ {
		var b strings.Builder
		for c, col := range cols {
			cell := ""
			if r < len(col) {
				cell = col[r]
			}
			if c < len(cols)-1 {
				b.WriteString(padCell(cell, colW))
				b.WriteString(gap)
			} else {
				b.WriteString(cell)
			}
		}
		rows[r] = b.String()
	}
	return strings.Join(rows, "\n")
}

func GlobalHelp() []HelpSection {
	return []HelpSection{
		{
			Title: "Global",
			Entries: []HelpEntry{
				{"1 2 3 4", "Switch tab: Home, Doctor, Config, Files"},
				{"?", "This help"},
				{"Esc", "Close overlay, or return to Home"},
				{"R", "Refresh the current tab"},
				{"S", "Open the setup wizard"},
				{"q", "Quit (ignored while editing a config field)"},
				{"Ctrl+C", "Quit from anywhere"},
			},
		},
		{
			Title: "Home",
			Entries: []HelpEntry{
				{"r", "Start or stop recording"},
				{"a", "Toggle auto-record"},
				{"y / n", "Confirm or dismiss the auto-record prompt"},
				{"↑ ↓", "Move through the sessions list"},
				{"[ ]", "Previous / next page"},
				{"t", "Transcribe the selected session, or add it to the queue"},
				{"s", "Stop transcription and clear the queue"},
				{"o", "Open the session folder"},
				{"p", "Play the recording"},
				{"f", "Choose the file manager"},
				{"d", "Delete the selected session"},
			},
		},
		{
			Title: "Doctor",
			Entries: []HelpEntry{
				{"i", "Install Whisper"},
				{"g", "Install GPU support"},
				{"PgUp/PgDn", "Scroll the install log"},
			},
		},
		{
			Title: "Config",
			Entries: []HelpEntry{
				{"↑ ↓", "Move between fields"},
				{"Tab", "Next section"},
				{"Enter", "Edit the field or apply a choice"},
				{"Esc", "Cancel the edit"},
				{"d d", "Models: delete a runtime or model to free space"},
			},
		},
		{
			Title: "Files",
			Entries: []HelpEntry{
				{"drag a file", "Transcribe any audio or video file"},
				{"a", "Type or paste a path instead"},
				{"c", "Copy the transcript to the clipboard"},
				{"PgUp/PgDn", "Scroll the transcript"},
				{"t / s", "Transcribe again / stop"},
				{"l", "Language for new files"},
				{"o / d", "Open the folder / remove from the list"},
			},
		},
	}
}
