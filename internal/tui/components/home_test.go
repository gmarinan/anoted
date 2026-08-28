package components

import (
	"testing"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
)

func TestTruncateClampsToTerminalCells(t *testing.T) {
	// Byte-slicing split multibyte characters and emitted invalid UTF-8.
	got := truncate("áéíóúñáéíóúñ", 8)
	if !utf8.ValidString(got) {
		t.Fatalf("truncate produced invalid UTF-8: %q", got)
	}
	if w := lipgloss.Width(got); w != 8 {
		t.Fatalf("width = %d, want the full 8-cell budget for %q", w, got)
	}
	if s := "corto"; truncate(s, 10) != s {
		t.Fatalf("short string was modified")
	}
	// The unit is display cells, not runes: wide characters must count double
	// or they overflow the column they were measured for.
	if w := lipgloss.Width(truncate("日本語のテキスト", 8)); w > 8 {
		t.Fatalf("wide runes exceeded budget: width = %d", w)
	}
}
