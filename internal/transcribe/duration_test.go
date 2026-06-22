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
