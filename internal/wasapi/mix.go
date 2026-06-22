//go:build windows

package wasapi

// MixS16 averages two s16le PCM buffers frame-by-frame into dst.
// Shorter input is treated as silence. dst must be large enough for the longest input.
func MixS16(dst, a, b []byte) []byte {
	n := len(a)
	if len(b) > n {
		n = len(b)
	}
	if cap(dst) < n {
		dst = make([]byte, n)
	} else {
		dst = dst[:n]
	}
	for i := 0; i < n; i += 2 {
		var av, bv int32
		if i+1 < len(a) {
			av = int32(int16(int(a[i]) | int(a[i+1])<<8))
		}
		if i+1 < len(b) {
			bv = int32(int16(int(b[i]) | int(b[i+1])<<8))
		}
		mixed := (av + bv) / 2
		if mixed > 32767 {
			mixed = 32767
		}
		if mixed < -32768 {
			mixed = -32768
		}
		dst[i] = byte(mixed)
		dst[i+1] = byte(mixed >> 8)
	}
	return dst
}

// DownmixToMono copies the first channel of interleaved s16le stereo into mono dst.
func DownmixToMono(dst, interleaved []byte, channels int) []byte {
	if channels <= 1 {
		if cap(dst) < len(interleaved) {
			dst = make([]byte, len(interleaved))
		} else {
			dst = dst[:len(interleaved)]
		}
		copy(dst, interleaved)
		return dst
	}
	frames := len(interleaved) / (2 * channels)
	need := frames * 2
	if cap(dst) < need {
		dst = make([]byte, need)
	} else {
		dst = dst[:need]
	}
	for i := 0; i < frames; i++ {
		off := i * 2 * channels
		dst[i*2] = interleaved[off]
		dst[i*2+1] = interleaved[off+1]
	}
	return dst
}
