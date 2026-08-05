package recorder

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestWAVWriterRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.wav")
	w, err := NewWAVWriter(path, 48000, 2)
	if err != nil {
		t.Fatalf("NewWAVWriter: %v", err)
	}
	pcm := []byte{0, 0, 1, 0, 255, 127, 0, 128}
	w.WritePCM(pcm)
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if len(out) != wavHeaderSize+len(pcm) {
		t.Fatalf("len %d want %d", len(out), wavHeaderSize+len(pcm))
	}
	if string(out[0:4]) != "RIFF" || string(out[8:12]) != "WAVE" {
		t.Fatal("missing RIFF/WAVE header")
	}
	// The sizes are the whole point of Close — a stale header is what made an
	// interrupted recording unplayable.
	if got := binary.LittleEndian.Uint32(out[40:44]); got != uint32(len(pcm)) {
		t.Fatalf("data size = %d, want %d", got, len(pcm))
	}
	if got := binary.LittleEndian.Uint32(out[4:8]); got != uint32(36+len(pcm)) {
		t.Fatalf("riff size = %d, want %d", got, 36+len(pcm))
	}
}

func TestWAVWriterStreamsBeforeClose(t *testing.T) {
	// PCM must already be on disk before Close, so an abnormal exit leaves a
	// recoverable file rather than nothing at all.
	path := filepath.Join(t.TempDir(), "partial.wav")
	w, err := NewWAVWriter(path, 16000, 1)
	if err != nil {
		t.Fatalf("NewWAVWriter: %v", err)
	}
	defer w.Close()
	big := make([]byte, 128*1024)
	w.WritePCM(big)

	st, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if st.Size() <= wavHeaderSize {
		t.Fatalf("nothing flushed to disk before Close: size %d", st.Size())
	}
	if got := w.Written(); got != int64(len(big)) {
		t.Fatalf("Written() = %d, want %d", got, len(big))
	}
}
