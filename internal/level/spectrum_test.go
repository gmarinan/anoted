package level

import (
	"math"
	"testing"
)

func TestBandsFromPCMSilence(t *testing.T) {
	buf := make([]byte, chunkBytes)
	bands := (&spectrumScratch{}).bandsFromPCM(buf)
	for i, b := range bands {
		if b != 0 {
			t.Fatalf("band %d should be silent, got %v", i, b)
		}
	}
}

func TestBandsFromPCMNotFlatOnTone(t *testing.T) {
	// ~440 Hz sine at 16 kHz, one chunk
	buf := make([]byte, chunkBytes)
	for i := 0; i < chunkSamples; i++ {
		v := int16(20000 * math.Sin(2*math.Pi*440*float64(i)/levelMeterSampleRate))
		buf[i*2] = byte(v)
		buf[i*2+1] = byte(v >> 8)
	}
	bands := (&spectrumScratch{}).bandsFromPCM(buf)
	peakIdx := 0
	peakVal := bands[0]
	for i, b := range bands {
		if b > peakVal {
			peakVal = b
			peakIdx = i
		}
	}
	// 440 Hz should energize low-mid bands, not uniformly all bands
	if peakVal < 0.1 {
		t.Fatalf("expected visible energy, got peak %v", peakVal)
	}
	lowSum := 0.0
	highSum := 0.0
	for i, b := range bands {
		if i < BandCount/3 {
			lowSum += b
		} else if i > 2*BandCount/3 {
			highSum += b
		}
	}
	if lowSum <= highSum {
		t.Fatalf("440 Hz should bias low-mid bands: low=%v high=%v peakIdx=%d", lowSum, highSum, peakIdx)
	}
}

func TestEmphasizeTransients(t *testing.T) {
	prev := []float64{0.1, 0.2, 0.3}
	cur := []float64{0.1, 0.35, 0.3}
	out := emphasizeTransients(prev, cur)
	if out[1] <= cur[1] {
		t.Fatalf("expected boost on changed band, got %v", out[1])
	}
}

func TestSmoothBands(t *testing.T) {
	prev := []float64{0.5, 0.2}
	next := []float64{0.8, 0.1}
	out := smoothBands(prev, next)
	if out[0] != 0.8 {
		t.Fatalf("attack on band 0: %v", out[0])
	}
	if out[1] >= 0.2 {
		t.Fatalf("release on band 1: %v", out[1])
	}
}
