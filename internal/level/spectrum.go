package level

import (
	"math"
	"math/cmplx"
)

// BandCount is the number of fixed frequency bars in the equalizer display.
const BandCount = 32

const (
	fftSize              = 256
	minBandHz            = 80.0
	maxBandDiv           = 2.5 // use up to Nyquist / 2.5
	levelMeterSampleRate = 16000
	chunkSamples         = 320
	chunkBytes           = chunkSamples * 2
)

// bandsFromPCM derives log-spaced frequency band levels from a mono s16le chunk.
// Levels are absolute (not re-normalized to the loudest band each frame) so bars
// keep moving under steady audio instead of freezing for seconds.
func bandsFromPCM(buf []byte) []float64 {
	samples := pcmWindow(buf, fftSize)
	if len(samples) < fftSize {
		return make([]float64, BandCount)
	}
	mags := fftMagnitudes(samples)
	raw := groupBands(mags, BandCount, levelMeterSampleRate, fftSize)
	out := make([]float64, BandCount)
	const bandGain = 18.0
	for i, b := range raw {
		v := b * bandGain
		if v > 1 {
			v = 1
		}
		out[i] = v
	}
	return out
}

// emphasizeTransients boosts bands that changed since the last chunk so the EQ reacts faster.
func emphasizeTransients(prev, cur []float64) []float64 {
	out := make([]float64, len(cur))
	for i, c := range cur {
		delta := 0.0
		if i < len(prev) {
			delta = math.Abs(c - prev[i])
		}
		v := c + delta*4
		if v > 1 {
			v = 1
		}
		out[i] = v
	}
	return out
}

func pcmWindow(buf []byte, n int) []float64 {
	out := make([]float64, 0, n)
	count := len(buf) / 2
	if count > n {
		count = n
	}
	start := len(buf)/2 - count
	if start < 0 {
		start = 0
	}
	for i := 0; i < count; i++ {
		off := start*2 + i*2
		if off+1 >= len(buf) {
			break
		}
		sample := int16(buf[off]) | int16(buf[off+1])<<8
		w := 1.0
		if count > 1 {
			t := float64(i) / float64(count-1)
			w = 0.5 - 0.5*math.Cos(2*math.Pi*t)
		}
		out = append(out, float64(sample)/32768.0*w)
	}
	for len(out) < n {
		out = append(out, 0)
	}
	return out[:n]
}

func fftMagnitudes(samples []float64) []float64 {
	n := len(samples)
	re := make([]complex128, n)
	for i, v := range samples {
		re[i] = complex(v, 0)
	}
	fftInPlace(re)
	mags := make([]float64, n/2+1)
	for i := range mags {
		mags[i] = cmplx.Abs(re[i]) / float64(n)
	}
	return mags
}

func groupBands(mags []float64, numBands, sampleRate, n int) []float64 {
	maxFreq := float64(sampleRate) / maxBandDiv
	bands := make([]float64, numBands)
	for b := 0; b < numBands; b++ {
		fLow := minBandHz * math.Pow(maxFreq/minBandHz, float64(b)/float64(numBands))
		fHigh := minBandHz * math.Pow(maxFreq/minBandHz, float64(b+1)/float64(numBands))
		binLow := int(fLow * float64(n) / float64(sampleRate))
		binHigh := int(fHigh * float64(n) / float64(sampleRate))
		if binLow < 1 {
			binLow = 1
		}
		if binHigh <= binLow {
			binHigh = binLow + 1
		}
		if binHigh > len(mags) {
			binHigh = len(mags)
		}
		var sum float64
		cnt := 0
		for i := binLow; i < binHigh; i++ {
			sum += mags[i] * mags[i]
			cnt++
		}
		if cnt > 0 {
			bands[b] = math.Sqrt(sum / float64(cnt))
		}
	}
	return bands
}

// fftInPlace is an in-place radix-2 Cooley–Tukey FFT (n must be power of 2).
func fftInPlace(x []complex128) {
	n := len(x)
	if n <= 1 {
		return
	}
	// bit-reversal permutation
	j := 0
	for i := 1; i < n; i++ {
		bit := n >> 1
		for j&bit != 0 {
			j ^= bit
			bit >>= 1
		}
		j ^= bit
		if i < j {
			x[i], x[j] = x[j], x[i]
		}
	}
	for length := 2; length <= n; length <<= 1 {
		ang := -2 * math.Pi / float64(length)
		wlen := cmplx.Exp(complex(0, ang))
		for i := 0; i < n; i += length {
			w := complex(1, 0)
			half := length / 2
			for k := 0; k < half; k++ {
				u := x[i+k]
				v := x[i+k+half] * w
				x[i+k] = u + v
				x[i+k+half] = u - v
				w *= wlen
			}
		}
	}
}

func smoothBands(prev, sample []float64) []float64 {
	if len(prev) != len(sample) {
		out := make([]float64, len(sample))
		copy(out, sample)
		return out
	}
	const release = 0.55 // faster fall (~20 ms parec chunks) for responsive bars
	out := make([]float64, len(sample))
	for i := range sample {
		if sample[i] > prev[i] {
			out[i] = sample[i]
		} else {
			out[i] = math.Max(sample[i], prev[i]*release)
		}
	}
	return out
}
