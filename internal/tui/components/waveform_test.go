package components

import (
	"math"
	"strings"
	"testing"
)

func TestScaleLevel(t *testing.T) {
	if scaleLevel(0) != 0 {
		t.Fatalf("silence should be 0, got %v", scaleLevel(0))
	}
	if scaleLevel(1) < 0.99 {
		t.Fatalf("full scale should map near 1, got %v", scaleLevel(1))
	}
}

func TestBandHeight(t *testing.T) {
	if bandHeight(0, 0, 0) != 0 {
		t.Fatal("silence")
	}
	h := bandHeight(0.3, 1, 0)
	if h < 2 || h > 6 {
		t.Fatalf("mid level should be mid-height, got %v", h)
	}
	if bandHeight(0.5, 1, 0) < bandHeight(0.1, 1, 0) {
		t.Fatal("louder should be taller")
	}
}

func TestFitBands(t *testing.T) {
	bands := []float64{0.1, 0.2, 0.3, 0.4}
	out := fitBands(bands, 4)
	if out[0] != 0.1 || out[3] != 0.4 {
		t.Fatalf("unexpected fit: %v", out)
	}
}

func TestRenderEqualizer(t *testing.T) {
	heights := []float64{2, 5, 8, 3, 6, 1, 4, 7}
	out := renderEqualizer(heights, systemEQColors)
	lines := strings.Split(stripANSI(out), "\n")
	if len(lines) != eqRows {
		t.Fatalf("expected %d rows, got %d", eqRows, len(lines))
	}
	if !strings.Contains(lines[len(lines)-1], "█") {
		t.Fatalf("bottom row should have blocks: %q", lines[len(lines)-1])
	}
}

func TestWaveformVizRender(t *testing.T) {
	bands := make([]float64, 32)
	for i := range bands {
		bands[i] = float64(i%5+1) * 0.12
	}
	v := WaveformViz{
		SystemBands:    bands,
		MicBands:       []float64{0.1, 0.4, 0.2, 0.35},
		SystemLabel:    "alsa_output.test",
		MicLabel:       "Built-in Mic",
		Recording:      true,
		LevelEnabled:   true,
		LevelAvailable: true,
		Width:          40,
		LevelFrame:     5,
	}
	out := v.Render()
	if !strings.Contains(out, "System audio") {
		t.Fatal("missing system label")
	}
}

func TestWaveformViewChangesEachFrame(t *testing.T) {
	bands := []float64{0.2, 0.3, 0.25, 0.4}
	v := WaveformViz{SystemBands: bands, Width: 30, LevelFrame: 1, LevelEnabled: true}
	a := stripANSI(v.Render())
	v.LevelFrame = 2
	b := stripANSI(v.Render())
	if a == b {
		t.Fatal("view should differ per frame via invisible sync marker")
	}
}

func TestWaveformVizMicIdle(t *testing.T) {
	v := WaveformViz{
		SystemBands:    []float64{0.1, 0.2},
		SystemLabel:    "out",
		MicLabel:       "mic",
		Recording:      false,
		LevelEnabled:   true,
		LevelAvailable: true,
		Width:          30,
	}
	out := v.Render()
	if !strings.Contains(out, "activo al grabar") {
		t.Fatal("expected idle mic hint")
	}
}

func stripANSI(s string) string {
	var b strings.Builder
	inEsc := false
	for i := 0; i < len(s); i++ {
		if s[i] == '\x1b' {
			inEsc = true
			continue
		}
		if inEsc {
			if s[i] == 'm' {
				inEsc = false
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func TestScaleLevelMonotonic(t *testing.T) {
	prev := -1.0
	for _, v := range []float64{1e-5, 1e-4, 1e-3, 0.01, 0.1, 0.5, 1.0} {
		s := scaleLevel(v)
		if s < prev {
			t.Fatalf("scale should be monotonic: v=%v s=%v prev=%v", v, s, prev)
		}
		prev = s
	}
	_ = math.Log10(0.1)
}
