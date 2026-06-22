package recorder

import (
	"encoding/binary"
)

// NewWAVWriter creates an incremental WAV writer for s16le PCM.
func NewWAVWriter(sampleRate, channels int) *WAVWriter {
	if sampleRate <= 0 {
		sampleRate = 48000
	}
	if channels <= 0 {
		channels = 2
	}
	return &WAVWriter{
		sampleRate: sampleRate,
		channels:   channels,
	}
}

// WAVWriter streams PCM samples into a WAV file layout in memory.
type WAVWriter struct {
	sampleRate int
	channels   int
	data       []byte
}

// WritePCM appends interleaved s16le samples.
func (w *WAVWriter) WritePCM(pcm []byte) {
	if len(pcm) == 0 {
		return
	}
	w.data = append(w.data, pcm...)
}

// Bytes returns the complete WAV file contents.
func (w *WAVWriter) Bytes() []byte {
	out := minimalWAVHeader(w.sampleRate, w.channels, len(w.data))
	out = append(out, w.data...)
	return out
}

func minimalWAVHeader(sampleRate, channels, dataSize int) []byte {
	bitsPerSample := 16
	byteRate := sampleRate * channels * bitsPerSample / 8
	blockAlign := channels * bitsPerSample / 8
	fileSize := 36 + dataSize

	buf := make([]byte, 44)
	copy(buf[0:4], "RIFF")
	binary.LittleEndian.PutUint32(buf[4:8], uint32(fileSize))
	copy(buf[8:12], "WAVE")
	copy(buf[12:16], "fmt ")
	binary.LittleEndian.PutUint32(buf[16:20], 16)
	binary.LittleEndian.PutUint16(buf[20:22], 1)
	binary.LittleEndian.PutUint16(buf[22:24], uint16(channels))
	binary.LittleEndian.PutUint32(buf[24:28], uint32(sampleRate))
	binary.LittleEndian.PutUint32(buf[28:32], uint32(byteRate))
	binary.LittleEndian.PutUint16(buf[32:34], uint16(blockAlign))
	binary.LittleEndian.PutUint16(buf[34:36], uint16(bitsPerSample))
	copy(buf[36:40], "data")
	binary.LittleEndian.PutUint32(buf[40:44], uint32(dataSize))
	return buf
}
