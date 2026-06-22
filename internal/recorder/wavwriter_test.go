package recorder

import "testing"

func TestWAVWriterRoundTrip(t *testing.T) {
	w := NewWAVWriter(48000, 2)
	pcm := []byte{0, 0, 1, 0, 255, 127, 0, 128}
	w.WritePCM(pcm)
	out := w.Bytes()
	if len(out) != 44+len(pcm) {
		t.Fatalf("len %d want %d", len(out), 44+len(pcm))
	}
	if string(out[0:4]) != "RIFF" {
		t.Fatal("missing RIFF header")
	}
}

func TestMixS16InRecorder(t *testing.T) {
	// MixS16 lives in wasapi; test minimal header only here.
	w := NewWAVWriter(16000, 1)
	w.WritePCM([]byte{0, 0})
	if len(w.Bytes()) < 44 {
		t.Fatal("wav too short")
	}
}
