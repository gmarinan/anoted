package level

import "testing"

// bandsFromPCM runs on every 20ms chunk from parec — 50 times a second per
// stream, and two streams while recording.
func BenchmarkBandsFromPCM(b *testing.B) {
	// One chunk of s16le mono at the level-meter rate.
	buf := make([]byte, chunkBytes)
	for i := 0; i+1 < len(buf); i += 2 {
		v := int16((i * 37) % 8000)
		buf[i] = byte(v)
		buf[i+1] = byte(v >> 8)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = (&spectrumScratch{}).bandsFromPCM(buf)
	}
}
