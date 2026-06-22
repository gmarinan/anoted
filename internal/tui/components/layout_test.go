package components

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestFloatCenterPreservesBackgroundBesideModal(t *testing.T) {
	background := "left text here          right text here\nsecond line left        second right"
	overlay := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Background(lipgloss.Color("235")).
		Padding(0, 1).
		Render("PICK\none")

	got := FloatCenter(background, overlay, 0, 0)
	if !strings.Contains(got, "left text") {
		t.Fatalf("expected left background text to remain, got:\n%s", got)
	}
	if !strings.Contains(got, "right text") {
		t.Fatalf("expected right background text to remain, got:\n%s", got)
	}
	if !strings.Contains(got, "second right") {
		t.Fatalf("expected second line background text to remain, got:\n%s", got)
	}
}
