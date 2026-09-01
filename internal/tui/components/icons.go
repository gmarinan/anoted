package components

// ASCII-safe status labels (no Nerd Font / PUA glyphs).
const (
	LabelRecording = "REC"
	// LabelWarn replaces U+26A0. That codepoint is East-Asian Ambiguous and most
	// emoji fonts render it two cells wide, while lipgloss.Width measures one —
	// which pushed the right border of every box containing a warning out by a
	// column.
	LabelWarn = "!"
)
