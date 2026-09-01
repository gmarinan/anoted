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

func TestUpdateBandsTransientBoostAndBallistics(t *testing.T) {
	smoothed := []float64{0.1, 0.5, 0.2}
	prev := []float64{0.1, 0.2, 0.1}
	cur := []float64{0.1, 0.35, 0.1}
	updateBands(smoothed, prev, cur)
	if smoothed[1] <= 0.35 {
		t.Fatalf("changed band should be boosted above its raw level, got %v", smoothed[1])
	}
	// Band 2 is steady and below its smoothed level: exponential release.
	if smoothed[2] >= 0.2 {
		t.Fatalf("quieter band should release, got %v", smoothed[2])
	}
	if smoothed[2] < 0.1 {
		t.Fatalf("release should not fall below the sample, got %v", smoothed[2])
	}
	for i, c := range cur {
		if prev[i] != c {
			t.Fatalf("prev[%d] should advance to cur, got %v", i, prev[i])
		}
	}
}

func TestFoldBandsFirstChunkAndSteadyState(t *testing.T) {
	cur := []float64{0.4, 0.2}
	smoothed, prev := foldBands(nil, nil, cur)
	if smoothed[0] != 0.4 || smoothed[1] != 0.2 {
		t.Fatalf("first chunk should show raw levels, got %v", smoothed)
	}
	if &smoothed[0] == &cur[0] {
		t.Fatal("smoothed must not alias the chunk scratch")
	}
	s2, p2 := foldBands(smoothed, prev, []float64{0.4, 0.2})
	if &s2[0] != &smoothed[0] || &p2[0] != &prev[0] {
		t.Fatal("steady state must reuse the monitor-owned buffers")
	}
	// A silent chunk after a steady one: the first fold spikes on the
	// transient, the second (no change) releases toward silence.
	s3, _ := foldBands(s2, p2, []float64{0.4, 0})
	spike := s3[1] // s3 is updated in place by the next fold; keep the value
	s4, _ := foldBands(s3, p2, []float64{0.4, 0})
	if s4[1] >= spike || s4[1] <= 0 {
		t.Fatalf("band 1 should release after silence, got %v then %v", spike, s4[1])
	}
}

// TestRealFFTMatchesNaiveDFT pins the packed real-input FFT (including the
// untangling pass and normalization) against a direct DFT of the same samples.
func TestRealFFTMatchesNaiveDFT(t *testing.T) {
	samples := make([]float64, fftSize)
	for i := range samples {
		samples[i] = math.Sin(0.37*float64(i)) * math.Cos(0.011*float64(i)*float64(i))
	}
	var s spectrumScratch
	copy(s.samples[:], samples)
	msq := realFFTMagSq(s.packed[:], s.magsSq[:], s.samples[:])

	for k := 0; k <= fftSize/2; k++ {
		var re, im float64
		for j, x := range samples {
			ang := -2 * math.Pi * float64(j) * float64(k) / float64(fftSize)
			re += x * math.Cos(ang)
			im += x * math.Sin(ang)
		}
		want := (re*re + im*im) / float64(fftSize) / float64(fftSize)
		if diff := math.Abs(msq[k] - want); diff > 1e-9 {
			t.Fatalf("bin %d: got %v want %v (diff %v)", k, msq[k], want, diff)
		}
	}
}
