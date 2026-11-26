package components

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestFitBoxRows(t *testing.T) {
	box := Box("Details", "one\ntwo\nthree\nfour", 30) // 6 rows
	cut := FitBoxRows(box, 4)
	if lipgloss.Height(cut) != 4 {
		t.Fatalf("cut to %d rows", lipgloss.Height(cut))
	}
	plain := ansi.Strip(cut)
	if !strings.Contains(plain, "one") || !strings.Contains(plain, "…") || strings.Contains(plain, "two") {
		t.Fatalf("cut box:\n%s", plain)
	}
	if !strings.HasPrefix(strings.Split(plain, "\n")[3], "╰") {
		t.Fatal("bottom border lost")
	}

	tall := FitBoxRows(box, 9)
	if lipgloss.Height(tall) != 9 || lipgloss.Width(tall) != 30 {
		t.Fatalf("stretched to %dx%d", lipgloss.Width(tall), lipgloss.Height(tall))
	}
	if FitBoxRows(box, 6) != box || FitBoxRows(box, 2) != box {
		t.Fatal("exact or impossible budgets should leave the box alone")
	}
}

func TestFooterWithStatusStaysOnOneLine(t *testing.T) {
	hints := []string{FooterHint("Enter", "edit"), FooterHint("←→", "section"), FooterHint("q", "quit")}
	for _, w := range []int{30, 60, 120} {
		for _, status := range []string{"✓ saved", "✗ " + strings.Repeat("a very long error ", 8)} {
			got := FitFooterWithStatus(w, status, hints...)
			if strings.Contains(got, "\n") || ansi.StringWidth(got) > w {
				t.Errorf("width %d: %q", w, ansi.Strip(got))
			}
		}
	}
}

func TestHelpPutsTheOpenTabFirst(t *testing.T) {
	v := HelpView{Sections: GlobalHelp(), Current: "Files", Width: 80, Height: 24}
	got := v.ordered()
	if got[0].Title != "Files" || got[1].Title != "Global" {
		t.Fatalf("order: %s, %s", got[0].Title, got[1].Title)
	}
}
