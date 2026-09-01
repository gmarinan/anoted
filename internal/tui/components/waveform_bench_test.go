package components

import "testing"

// BenchmarkWaveformRender measures one full-width equalizer paint, the piece
// of the frame that redraws most often while audio plays.
func BenchmarkWaveformRender(b *testing.B) {
	bands := make([]float64, 32)
	for i := range bands {
		bands[i] = float64(i%5+1) * 0.12
	}
	v := WaveformViz{
		SystemBands:    bands,
		MicBands:       bands,
		SystemLabel:    "alsa_output.test",
		MicLabel:       "Built-in Mic",
		Recording:      true,
		LevelEnabled:   true,
		LevelAvailable: true,
		Width:          100,
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = v.Render()
	}
}
