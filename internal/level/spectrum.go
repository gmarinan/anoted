package level

import "math"

// BandCount is the number of fixed frequency bars in the equalizer display.
const BandCount = 32

const (
	fftSize              = 256
	minBandHz            = 80.0
	maxBandDiv           = 2.5 // use up to Nyquist / 2.5
	levelMeterSampleRate = 16000
	chunkSamples         = 320
	chunkBytes           = chunkSamples * 2

	// bandRelease is the per-chunk exponential fall applied when a band gets
	// quieter (~20 ms parec chunks); attack is instant.
	bandRelease = 0.55
)

// spectrumScratch holds the working buffers for one audio stream.
//
// Each parec reader owns one. They cannot be package-level: system and
// microphone capture run concurrently while recording, and sharing the buffers
// between them would be a data race.
type spectrumScratch struct {
	samples [fftSize]float64
	packed  [fftSize / 2]complex128
	magsSq  [fftSize/2 + 1]float64
	out     [BandCount]float64
}

// bandsFromPCM derives log-spaced frequency band levels from a mono s16le chunk.
// Levels are absolute (not re-normalized to the loudest band each frame) so bars
// keep moving under steady audio instead of freezing for seconds.
//
// The returned slice aliases the scratch and is overwritten by the next chunk;
// callers must consume or copy it before then (foldBands copies under lock).
func (s *spectrumScratch) bandsFromPCM(buf []byte) []float64 {
	samples := pcmWindow(s.samples[:], buf)
	msq := realFFTMagSq(s.packed[:], s.magsSq[:], samples)
	out := groupBands(s.out[:], msq)
	const bandGain = 18.0
	for i, b := range out {
		v := b * bandGain
		if v > 1 {
			v = 1
		}
		out[i] = v
	}
	return out
}

// updateBands folds one chunk of band levels into monitor-owned display state:
// transient emphasis against prev, then instant-attack/exponential-release
// smoothing into smoothed. All slices must be the same length; prev is
// advanced to cur. Fused into one pass so the chunk path allocates nothing.
func updateBands(smoothed, prev, cur []float64) {
	for i, c := range cur {
		// Boost bands that changed since the last chunk so the EQ reacts faster.
		e := c + math.Abs(c-prev[i])*4
		if e > 1 {
			e = 1
		}
		if e > smoothed[i] {
			smoothed[i] = e
		} else {
			smoothed[i] = math.Max(e, smoothed[i]*bandRelease)
		}
		prev[i] = c
	}
}

// foldBands merges one chunk into the monitor's smoothed/prev band state,
// allocating only on the first chunk after a (re)start. cur may alias scratch;
// only its values are retained.
func foldBands(smoothed, prev, cur []float64) (s, p []float64) {
	if len(prev) != len(cur) {
		prev = make([]float64, len(cur))
	}
	if len(smoothed) != len(cur) {
		// First chunk: no previous state, so no transient boost and nothing to
		// release — the display simply starts at this chunk's levels.
		smoothed = append([]float64(nil), cur...)
		copy(prev, cur)
		return smoothed, prev
	}
	updateBands(smoothed, prev, cur)
	return smoothed, prev
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

// realFFTMagSq computes the squared, normalized magnitude spectrum of fftSize
// real samples using a half-size complex FFT: the even/odd samples are packed
// into fftSize/2 complex values, transformed, then untangled bin by bin. That
// halves the FFT work versus transforming a zero-imaginary complex input, and
// squared magnitudes let groupBands skip a hypot per bin (it averages powers
// anyway).
func realFFTMagSq(packed []complex128, msq []float64, samples []float64) []float64 {
	const n = fftSize / 2
	packed = packed[:n]
	for k := 0; k < n; k++ {
		packed[k] = complex(samples[2*k], samples[2*k+1])
	}
	fftInPlace(packed)

	msq = msq[:n+1]
	const invN2 = 1.0 / float64(fftSize) / float64(fftSize)
	// DC and Nyquist bins are real-valued combinations of the first packed bin.
	re0, im0 := real(packed[0]), imag(packed[0])
	msq[0] = (re0 + im0) * (re0 + im0) * invN2
	msq[n] = (re0 - im0) * (re0 - im0) * invN2
	for k := 1; k < n; k++ {
		zk := packed[k]
		znk := packed[n-k]
		// Even/odd untangling: X[k] = Fe[k] + W^k·Fo[k] with W = e^(-2πi/fftSize).
		fe := complex((real(zk)+real(znk))/2, (imag(zk)-imag(znk))/2)
		fo := complex((imag(zk)+imag(znk))/2, (real(znk)-real(zk))/2)
		x := fe + rfftTwiddles[k]*fo
		msq[k] = (real(x)*real(x) + imag(x)*imag(x)) * invN2
	}
	return msq
}

// groupBands collapses the power spectrum into the display bands using the
// precomputed bin boundaries: RMS magnitude per band.
func groupBands(dst []float64, msq []float64) []float64 {
	dst = dst[:BandCount]
	for b, r := range bandBins {
		low, high := r[0], r[1]
		if high > len(msq) {
			high = len(msq)
		}
		var sum float64
		for i := low; i < high; i++ {
			sum += msq[i]
		}
		if cnt := high - low; cnt > 0 {
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
