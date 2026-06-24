package components

import (
	"strings"
	"testing"
)

func TestWaveformVizDisabled(t *testing.T) {
	v := WaveformViz{
		SystemLabel:  "alsa_output.test",
		MicLabel:     "Built-in Mic",
		LevelEnabled: false,
		Width:        40,
	}
	out := stripANSI(v.Render())
	if strings.Contains(out, "█") || strings.Contains(out, "activo al grabar") {
		t.Fatalf("disabled meter should not render bars: %q", out)
	}
	if !strings.Contains(out, "alsa") {
		t.Fatal("should still show device labels")
	}
}
