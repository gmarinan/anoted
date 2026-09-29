package components

import (
	"strings"
	"testing"
	"time"

	"anoted/internal/session"
)

func TestSessionsViewShowsDiskUsage(t *testing.T) {
	dir := "/tmp/sess"
	v := SessionsView{
		Width:      100,
		Height:     40,
		TotalCount: 1,
		Page:       1,
		PageCount:  1,
		PageRecords: []session.Record{{
			ID:        1,
			Dir:       dir,
			StartedAt: time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC),
			Provider:  "google_meet",
		}},
		AudioUsage: "1.5 GB · 4 files · 200.0 MB not transcribed",
		Artifacts: map[string]SessionArtifacts{
			dir: {HasTranscript: true},
		},
	}
	got := v.RenderMainContent()
	if !strings.Contains(got, "1.5 GB") {
		t.Fatalf("disk usage missing:\n%s", got)
	}
	if !strings.Contains(got, "removed — transcript kept") {
		t.Fatalf("removed audio not explained:\n%s", got)
	}
}

func TestStatusBoxShowsDiskUsage(t *testing.T) {
	v := HomeView{
		Width:      80,
		AppState:   "idle",
		Provider:   "None detected",
		AudioUsage: "12.0 MB · 2 files",
	}
	got := v.StatusBox(40)
	if !strings.Contains(got, "12.0 MB") {
		t.Fatalf("status disk line missing:\n%s", got)
	}
	hidden := HomeView{Width: 80, AppState: "idle"}.StatusBox(40)
	if strings.Contains(hidden, "Disk") {
		t.Fatal("disk row should stay hidden until the library has been measured")
	}
}
