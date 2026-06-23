package components

import (
	"strings"
	"testing"
)

func TestConfigMenuSidebarLayout(t *testing.T) {
	v := ConfigMenuView{
		Path: "/home/user/.config/anoted/config.yaml",
		Width: 100,
		Sections: []ConfigSectionPanel{
			{Label: "General", Focused: true, Fields: []ConfigFieldRow{
				{Label: "output_dir", Value: "/tmp", Selected: true, Kind: "text"},
			}},
			{Label: "Audio"},
			{Label: "Detection"},
		},
	}
	got := v.View()
	if !strings.Contains(got, "GENERAL") {
		t.Fatalf("expected sidebar label GENERAL, got:\n%s", got)
	}
	if !strings.Contains(got, "output_dir") {
		t.Fatalf("expected active section field, got:\n%s", got)
	}
	if strings.Count(got, "AUDIO") < 1 {
		t.Fatalf("expected AUDIO in sidebar, got:\n%s", got)
	}
}

func TestConfigMenuStackedOnNarrowWidth(t *testing.T) {
	v := ConfigMenuView{
		Path:  "/cfg.yaml",
		Width: 60,
		Sections: []ConfigSectionPanel{
			{Label: "General", Focused: true, Fields: []ConfigFieldRow{
				{Label: "auto_record", Value: "false", Kind: "bool"},
			}},
			{Label: "Audio"},
		},
	}
	got := v.View()
	if !strings.Contains(got, "[General]") {
		t.Fatalf("expected horizontal section tabs on narrow width, got:\n%s", got)
	}
}
