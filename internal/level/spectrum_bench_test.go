package level

import "testing"

// bandsFromPCM runs on every parec read — up to 50 times a second per stream,
// and two streams while recording. The scratch is long-lived per stream, so it
// is hoisted out of the loop exactly as in linuxMonitor.startStream.
func BenchmarkBandsFromPCM(b *testing.B) {
	buf := benchChunk()
	scratch := &spectrumScratch{}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = scratch.bandsFromPCM(buf)
	}
}

// BenchmarkChunkFold measures the whole steady-state per-chunk pipeline as the
// reader goroutine runs it: peak, FFT bands, and the fold into monitor state.
func BenchmarkChunkFold(b *testing.B) {
	buf := benchChunk()
	scratch := &spectrumScratch{}
	var smoothed, prev []float64
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = peakS16LE(buf)
		smoothed, prev = foldBands(smoothed, prev, scratch.bandsFromPCM(buf))
	}
}

func benchChunk() []byte {
	buf := make([]byte, chunkBytes)
	for i := 0; i+1 < len(buf); i += 2 {
		v := int16((i * 37) % 8000)
		buf[i] = byte(v)
		buf[i+1] = byte(v >> 8)
	}
	return buf
}
