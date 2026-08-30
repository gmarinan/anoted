package level

import (
	"math"
	"math/cmplx"
)

// Everything in this file is derived from compile-time constants — fftSize,
// BandCount, levelMeterSampleRate — and was being recomputed on every 20ms
// audio chunk. parec delivers 50 chunks a second per stream, and two streams
// run while recording, so this was ~25,600 math.Cos and ~6,400 math.Pow calls
// per second producing the same numbers every time, plus a fresh set of slices
// for the garbage collector.

// hannWindow is the analysis window applied before the FFT.
var hannWindow = func() [fftSize]float64 {
	var w [fftSize]float64
	for i := range w {
		t := float64(i) / float64(fftSize-1)
		w[i] = 0.5 - 0.5*math.Cos(2*math.Pi*t)
	}
	return w
}()

// bandBins holds the [low, high) magnitude-bin range for each display band.
// The boundaries are log-spaced between minBandHz and Nyquist/maxBandDiv.
var bandBins = func() [BandCount][2]int {
	var bins [BandCount][2]int
	maxFreq := float64(levelMeterSampleRate) / maxBandDiv
	magCount := fftSize/2 + 1
	for b := 0; b < BandCount; b++ {
		fLow := minBandHz * math.Pow(maxFreq/minBandHz, float64(b)/float64(BandCount))
		fHigh := minBandHz * math.Pow(maxFreq/minBandHz, float64(b+1)/float64(BandCount))
		low := int(fLow * float64(fftSize) / float64(levelMeterSampleRate))
		high := int(fHigh * float64(fftSize) / float64(levelMeterSampleRate))
		if low < 1 {
			low = 1
		}
		if high <= low {
			high = low + 1
		}
		if high > magCount {
			high = magCount
		}
		bins[b] = [2]int{low, high}
	}
	return bins
}()

// twiddles[s] holds the roots of unity for the FFT stage of length 1<<s.
// cmplx.Exp was being called once per stage per chunk for the same eight values.
var twiddles = func() [fftStages]complex128 {
	var t [fftStages]complex128
	for s := 1; s <= fftStages; s++ {
		length := 1 << s
		t[s-1] = cmplx.Exp(complex(0, -2*math.Pi/float64(length)))
	}
	return t
}()

// fftStages is log2(fftSize).
const fftStages = 8

func init() {
	// Guard the constant relationship the tables above bake in.
	if 1<<fftStages != fftSize {
		panic("fftStages must be log2(fftSize)")
	}
}
