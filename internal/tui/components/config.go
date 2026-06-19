package components

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
)

// ConfigView renders the Config tab YAML editor.
type ConfigView struct {
	Path      string
	Lines     []string
	CursorRow int
	CursorCol int // rune index in line
	ScrollRow int
	Dirty     bool
	ErrMsg    string
	SavedMsg  string
	Width     int
	Height    int
}

func (v ConfigView) View() string {
	var b strings.Builder
	b.WriteString(Header())
	b.WriteString("\n")
	b.WriteString(TabBar(TabConfig))
	if v.Dirty {
		b.WriteString(warnStyle.Render(" ●"))
	}
	b.WriteString("\n")
	b.WriteString(subtleStyle.Render("File: "+v.Path))
	b.WriteString("\n\n")

	if v.ErrMsg != "" {
		b.WriteString(errStyle.Render("✗ " + v.ErrMsg))
		b.WriteString("\n")
	}
	if v.SavedMsg != "" {
		b.WriteString(okStyle.Render("✓ " + v.SavedMsg))
		b.WriteString("\n")
	}

	editorH := v.editorHeight()
	content := v.renderEditor(editorH)
	b.WriteString(Box("config.yaml", content, v.Width))
	return b.String()
}

func (v ConfigView) editorHeight() int {
	h := v.Height - 9
	if h < 6 {
		h = 6
	}
	return h
}

func (v ConfigView) renderEditor(visibleLines int) string {
	if len(v.Lines) == 0 {
		return subtleStyle.Render("(empty)")
	}

	var out []string
	for i := 0; i < visibleLines; i++ {
		lineIdx := v.ScrollRow + i
		if lineIdx >= len(v.Lines) {
			out = append(out, "")
			continue
		}
		line := v.Lines[lineIdx]
		prefix := "  "
		if lineIdx == v.CursorRow {
			prefix = "> "
		}
		display := v.formatLine(line, lineIdx)
		num := subtleStyle.Render(fmt.Sprintf("%3d ", lineIdx+1))
		out = append(out, num+prefix+display)
	}
	return strings.Join(out, "\n")
}

func (v ConfigView) formatLine(line string, lineIdx int) string {
	maxW := v.Width - 10
	if maxW < 20 {
		maxW = 40
	}
	runes := []rune(line)
	if lineIdx == v.CursorRow {
		return v.renderCursorLine(runes, maxW)
	}
	return valueStyle.Render(truncateRunes(runes, maxW))
}

func (v ConfigView) renderCursorLine(runes []rune, maxW int) string {
	col := v.CursorCol
	if col > len(runes) {
		col = len(runes)
	}
	start := 0
	if col > maxW-1 {
		start = col - maxW + 1
	}
	end := start + maxW
	if end > len(runes) {
		end = len(runes)
	}
	visible := runes[start:end]
	var b strings.Builder
	cursorStyle := lipgloss.NewStyle().Background(lipgloss.Color("63")).Foreground(lipgloss.Color("229"))
	for i, r := range visible {
		pos := start + i
		if pos == col {
			b.WriteString(cursorStyle.Render(string(r)))
		} else {
			b.WriteString(valueStyle.Render(string(r)))
		}
	}
	if col == len(runes) {
		b.WriteString(cursorStyle.Render(" "))
	}
	return b.String()
}

func truncateRunes(runes []rune, max int) string {
	if len(runes) <= max {
		return string(runes)
	}
	return string(runes[:max-1]) + "…"
}

// LinesToText joins editor lines for saving.
func LinesToText(lines []string) string {
	return strings.Join(lines, "\n")
}

// TextToLines splits file content into editor lines.
func TextToLines(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	if text == "" {
		return []string{""}
	}
	lines := strings.Split(text, "\n")
	return lines
}

// ClampCursorCol ensures the column is valid for the current line.
func ClampCursorCol(lines []string, row, col int) int {
	if row < 0 || row >= len(lines) {
		return 0
	}
	n := utf8.RuneCountInString(lines[row])
	if col < 0 {
		return 0
	}
	if col > n {
		return n
	}
	return col
}
