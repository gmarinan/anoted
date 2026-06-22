package level

import "math"

const decayRate = 0.85

// peakS16LE returns the normalized peak amplitude in 0..1 from little-endian s16 PCM.
func peakS16LE(buf []byte) float64 {
	if len(buf) < 2 {
		return 0
	}
	var peak int32
	for i := 0; i+1 < len(buf); i += 2 {
		sample := int32(int16(buf[i]) | int16(buf[i+1])<<8)
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
