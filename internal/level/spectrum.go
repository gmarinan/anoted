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
// spectrumScratch holds the working buffers for one audio stream.
//
// Each parec reader owns one. They cannot be package-level: system and
// microphone capture run concurrently while recording, and sharing the buffers
// between them would be a data race.
type spectrumScratch struct {
	samples [fftSize]float64
	complex [fftSize]complex128
	mags    [fftSize/2 + 1]float64
	bands   [BandCount]float64
	out     [BandCount]float64
}

func (s *spectrumScratch) bandsFromPCM(buf []byte) []float64 {
	samples := pcmWindow(s.samples[:], buf)
	mags := fftMagnitudes(s.complex[:], s.mags[:], samples)
	raw := groupBands(s.bands[:], mags)

	// The result is handed to the caller, which stores it on the monitor and
	// later copies it out under lock, so it must not alias the scratch that the
	// next chunk overwrites.
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

// pcmWindow fills dst with the windowed tail of buf. dst is reused across
// chunks; the window itself is precomputed (see spectrum_tables.go).
func pcmWindow(dst []float64, buf []byte) []float64 {
	dst = dst[:fftSize]
	count := len(buf) / 2
	if count > fftSize {
		count = fftSize
	}
	start := len(buf)/2 - count
	if start < 0 {
		start = 0
	}
	for i := 0; i < count; i++ {
		off := start*2 + i*2
		if off+1 >= len(buf) {
			count = i
			break
		}
		sample := int16(buf[off]) | int16(buf[off+1])<<8
		dst[i] = float64(sample) / 32768.0 * hannWindow[i]
	}
	for i := count; i < fftSize; i++ {
		dst[i] = 0
	}
	return dst
}

// fftMagnitudes writes the magnitude spectrum of samples into mags, reusing the
// caller's complex scratch buffer.
func fftMagnitudes(scratch []complex128, mags []float64, samples []float64) []float64 {
	scratch = scratch[:fftSize]
	for i, v := range samples {
		scratch[i] = complex(v, 0)
	}
	fftInPlace(scratch)
	mags = mags[:fftSize/2+1]
	for i := range mags {
		mags[i] = cmplx.Abs(scratch[i]) / float64(fftSize)
	}
	return mags
}

// groupBands collapses the magnitude spectrum into the display bands using the
// precomputed bin boundaries.
func groupBands(dst []float64, mags []float64) []float64 {
	dst = dst[:BandCount]
	for b, r := range bandBins {
		low, high := r[0], r[1]
		if high > len(mags) {
			high = len(mags)
		}
		var sum float64
		cnt := 0
		for i := low; i < high; i++ {
			sum += mags[i] * mags[i]
			cnt++
		}
		if cnt > 0 {
			dst[b] = math.Sqrt(sum / float64(cnt))
		} else {
			dst[b] = 0
		}
	}
	return dst
}

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
	for stage, length := 1, 2; length <= n; stage, length = stage+1, length<<1 {
		wlen := twiddles[stage-1]
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
