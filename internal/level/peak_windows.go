package level

import "math"

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
