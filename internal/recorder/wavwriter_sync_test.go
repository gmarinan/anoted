package recorder

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// A recording killed before Close (crash, power loss, taskkill) used to leave a
// header declaring dataSize = 0, which every player and whisper reads as an
// empty file. The header must be refreshed as the recording runs.
func TestWAVHeaderIsUsableWithoutClose(t *testing.T) {
	path := filepath.Join(t.TempDir(), "recording.wav")
	const (
		rate     = 8000
		channels = 1
	)
	w, err := NewWAVWriter(path, rate, channels)
	if err != nil {
		t.Fatalf("NewWAVWriter: %v", err)
	}

	// Write well past one sync interval without ever calling Close.
	chunk := make([]byte, 4096)
	bytesPerSecond := rate * channels * 2
	total := 0
	for total < bytesPerSecond*(headerSyncSeconds+2) {
		w.WritePCM(chunk)
		total += len(chunk)
	}
	if err := w.Err(); err != nil {
		t.Fatalf("writer reported an error: %v", err)
	}

	header := make([]byte, wavHeaderSize)
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()
	if _, err := f.ReadAt(header, 0); err != nil {
		t.Fatalf("read header: %v", err)
	}

	dataSize := binary.LittleEndian.Uint32(header[40:44])
	if dataSize == 0 {
		t.Fatal("dataSize is still 0; an interrupted recording would be unplayable")
	}
	if int(dataSize) > total {
		t.Fatalf("dataSize %d exceeds the %d bytes written", dataSize, total)
	}
	// The sync happens on interval boundaries, so the header may lag by less
	// than one interval's worth of audio, never more.
	if lag := total - int(dataSize); lag > bytesPerSecond*headerSyncSeconds {
		t.Fatalf("header lags by %d bytes, more than one %ds sync interval", lag, headerSyncSeconds)
	}
	if riff := binary.LittleEndian.Uint32(header[4:8]); riff != dataSize+36 {
		t.Fatalf("RIFF size %d is inconsistent with dataSize %d", riff, dataSize)
	}
}

func TestWAVCloseStillReportsExactSize(t *testing.T) {
	path := filepath.Join(t.TempDir(), "recording.wav")
	w, err := NewWAVWriter(path, 8000, 1)
	if err != nil {
		t.Fatalf("NewWAVWriter: %v", err)
	}
	payload := make([]byte, 100_000)
	w.WritePCM(payload)
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got := binary.LittleEndian.Uint32(body[40:44]); int(got) != len(payload) {
		t.Fatalf("dataSize = %d, want %d", got, len(payload))
	}
	if len(body) != wavHeaderSize+len(payload) {
		t.Fatalf("file size = %d, want %d", len(body), wavHeaderSize+len(payload))
	}
}
