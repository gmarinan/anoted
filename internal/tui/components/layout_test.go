package components

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestPanelLayoutTwoColumn(t *testing.T) {
	wide := NewPanelLayout(100)
	if !wide.TwoColumn() || wide.ColumnWidth() != 49 {
		t.Fatalf("wide layout: twoCol=%v colW=%d", wide.TwoColumn(), wide.ColumnWidth())
	}

	narrow := NewPanelLayout(70)
	if narrow.TwoColumn() || narrow.ColumnWidth() != 70 {
		t.Fatalf("narrow layout: twoCol=%v colW=%d", narrow.TwoColumn(), narrow.ColumnWidth())
	}
}

func TestPanelLayoutJoinColumnsStacksWhenNarrow(t *testing.T) {
	layout := NewPanelLayout(60)
	left := Box("Left", "alpha", 60)
	right := Box("Right", "beta", 60)
	got := layout.JoinColumns(left, right)
	if !strings.Contains(got, "LEFT") || !strings.Contains(got, "RIGHT") {
		t.Fatalf("missing panels: %q", got)
	}
	if strings.Count(got, "╭") < 2 {
		t.Fatalf("expected stacked boxes, got:\n%s", got)
	}
}

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
