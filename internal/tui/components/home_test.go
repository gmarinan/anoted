package components

import (
	"testing"
	"unicode/utf8"
)

func TestTruncateCountsRunes(t *testing.T) {
	// Byte-slicing split multibyte characters and emitted invalid UTF-8.
	got := truncate("áéíóúñáéíóúñ", 8)
	if !utf8.ValidString(got) {
		t.Fatalf("truncate produced invalid UTF-8: %q", got)
	}
	if n := utf8.RuneCountInString(got); n != 6 {
		t.Fatalf("rune count = %d, want 6 (5 + ellipsis) for %q", n, got)
	}
	if s := "corto"; truncate(s, 10) != s {
		t.Fatalf("short string was modified")
	}
}
