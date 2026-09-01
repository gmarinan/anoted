package tui

import "testing"

func benchViewModel() Model {
	bands := make([]float64, 32)
	for i := range bands {
		bands[i] = float64(i%5+1) * 0.12
	}
	return Model{
		screen:      ScreenMain,
		width:       120,
		height:      40,
		sessions:    testSessions(20),
		scroll:      newSessionScroll(),
		cache:       &viewCache{},
		systemBands: bands,
		micBands:    bands,
		recording:   true,
	}
}

// BenchmarkViewHome measures a steady-state Home frame with a live meter — the
// hottest View path (up to 30 fps while audio plays). The model matches what
// NewModel builds, including the render cache, so repeated frames hit it just
// as they do between level ticks.
func BenchmarkViewHome(b *testing.B) {
	m := benchViewModel()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = m.View()
	}
}

// BenchmarkViewHomeCold renders with the cache invalidated every frame — the
// worst case, hit when sessions, cursor, size or transcription state change.
func BenchmarkViewHomeCold(b *testing.B) {
	m := benchViewModel()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		m.viewGen++ // force sessions-block and footer recompute
		_ = m.View()
	}
}
