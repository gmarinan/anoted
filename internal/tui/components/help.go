package components

import "strings"

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
	Width    int
	Height   int
}

// Overlay floats the shortcut list above the current screen.
func (v HelpView) Overlay(base string) string {
	var b strings.Builder
	for i, section := range v.Sections {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(boxTitleStyle.Render(section.Title))
		b.WriteString("\n")
		for _, e := range section.Entries {
			b.WriteString("  ")
			b.WriteString(padCell(keyStyle.Render(e.Key), helpKeyColumn))
			b.WriteString(subtleStyle.Render(e.Action))
			b.WriteString("\n")
		}
	}
	b.WriteString("\n")
	b.WriteString(subtleStyle.Render("Press ? or Esc to close"))

	modal := PickerModal("Keyboard shortcuts", strings.TrimRight(b.String(), "\n"), helpModalMaxWidth(v.Width))
	return FloatCenter(base, modal, v.Width, v.Height)
}

const helpKeyColumn = 14

func helpModalMaxWidth(width int) int {
	const preferred = 64
	if width > 0 && width-8 < preferred {
		return max(width-8, 24)
	}
	return preferred
}

// GlobalHelp is the keyboard map shown by the ? overlay. It is the single
// source of truth the README table is generated from.
func GlobalHelp() []HelpSection {
	return []HelpSection{
		{
			Title: "Global",
			Entries: []HelpEntry{
				{"1 2 3", "Switch tab: Home, Doctor, Config"},
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
				{"t", "Transcribe the selected session"},
				{"s", "Stop a running transcription"},
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
			},
		},
	}
}
