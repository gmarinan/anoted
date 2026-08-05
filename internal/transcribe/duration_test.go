package transcribe

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"anoted/internal/recorder"
)

func TestAudioDuration(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, recorder.SessionAudioFile)
	wav := minimalTestWAV(48000, 2, 48000) // 1 second stereo
	if err := os.WriteFile(path, wav, 0o644); err != nil {
		t.Fatal(err)
	}
	dur, err := AudioDuration(path)
	if err != nil {
		t.Fatal(err)
	}
	if dur < 900*time.Millisecond || dur > 1100*time.Millisecond {
		t.Fatalf("duration %v, want ~1s", dur)
	}
}

func minimalTestWAV(sampleRate, channels, samples int) []byte {
	bitsPerSample := 16
	byteRate := sampleRate * channels * bitsPerSample / 8
	blockAlign := channels * bitsPerSample / 8
	dataSize := samples * blockAlign
	fileSize := 36 + dataSize
	buf := make([]byte, 44+dataSize)
	copy(buf[0:4], "RIFF")
	putLE32(buf[4:8], uint32(fileSize))
	copy(buf[8:12], "WAVE")
	copy(buf[12:16], "fmt ")
	putLE32(buf[16:20], 16)
	putLE16(buf[20:22], 1)
	putLE16(buf[22:24], uint16(channels))
	putLE32(buf[24:28], uint32(sampleRate))
	putLE32(buf[28:32], uint32(byteRate))
	putLE16(buf[32:34], uint16(blockAlign))
	putLE16(buf[34:36], uint16(bitsPerSample))
	copy(buf[36:40], "data")
	putLE32(buf[40:44], uint32(dataSize))
	return buf
}

func putLE16(b []byte, v uint16) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
}

func putLE32(b []byte, v uint32) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
}

// wavWithLeadingChunk builds a WAV whose data chunk is preceded by a LIST
// chunk, the layout ffmpeg writes for every recording anoted makes on Linux.
func wavWithLeadingChunk(t *testing.T, sampleRate, channels, seconds int) string {
	t.Helper()
	dataSize := sampleRate * channels * 2 * seconds
	// INFO + ISFT + uint32 length + a NUL-terminated encoder string.
	listBody := []byte("INFOISFT\x0e\x00\x00\x00Lavf60.16.100\x00")

	var b []byte
	put32 := func(v int) { b = append(b, byte(v), byte(v>>8), byte(v>>16), byte(v>>24)) }
	put16 := func(v int) { b = append(b, byte(v), byte(v>>8)) }

	b = append(b, "RIFF"...)
	put32(4 + 8 + 16 + 8 + len(listBody) + 8 + dataSize)
	b = append(b, "WAVE"...)

	b = append(b, "fmt "...)
	put32(16)
	put16(1)
	put16(channels)
	put32(sampleRate)
	put32(sampleRate * channels * 2)
	put16(channels * 2)
	put16(16)

	b = append(b, "LIST"...)
	put32(len(listBody))
	b = append(b, listBody...)

	b = append(b, "data"...)
	put32(dataSize)
	b = append(b, make([]byte, dataSize)...)

	path := filepath.Join(t.TempDir(), "ffmpeg-style.wav")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatalf("write wav: %v", err)
	}
	return path
}

func TestAudioDurationSkipsLeadingChunks(t *testing.T) {
	// Reading at fixed offsets picked up the LIST chunk's size as the sample
	// count, reporting a 3-second file as ~125µs — which pinned the
	// transcription progress bar at 100% and wrote duration: 0s to the note.
	path := wavWithLeadingChunk(t, 48000, 2, 3)
	got, err := AudioDuration(path)
	if err != nil {
		t.Fatalf("AudioDuration: %v", err)
	}
	if got < 2900*time.Millisecond || got > 3100*time.Millisecond {
		t.Fatalf("AudioDuration = %v, want ~3s", got)
	}
}
