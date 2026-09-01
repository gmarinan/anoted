package level

import (
	"encoding/binary"
	"math"
)

const decayRate = 0.85

// peakS16LE returns the normalized peak amplitude in 0..1 from little-endian s16 PCM.
func peakS16LE(buf []byte) float64 {
	if len(buf) < 2 {
		return 0
	}
	var peak int32
	for i := 0; i+1 < len(buf); i += 2 {
		sample := int32(int16(binary.LittleEndian.Uint16(buf[i:])))
		if sample < 0 {
			sample = -sample
		}
		if sample > peak {
			peak = sample
		}
	}
	return float64(peak) / 32768.0
}

// smoothPeak applies attack/release smoothing for VU-style meters.
func smoothPeak(prev, sample float64) float64 {
	if sample > prev {
		return sample
	}
	return math.Max(sample, prev*decayRate)
}

// peakBands folds a flat peak level into the smoothed band state for
// lightweight recording feeds, reusing prev so the steady state allocates
// nothing. Same instant-attack/exponential-release ballistics as updateBands.
func peakBands(prev []float64, peak float64) []float64 {
	if len(prev) != BandCount {
		out := make([]float64, BandCount)
		for i := range out {
			out[i] = peak
		}
		return out
	}
	for i, p := range prev {
		if peak > p {
			prev[i] = peak
		} else {
			prev[i] = math.Max(peak, p*bandRelease)
		}
	}
	return prev
}
