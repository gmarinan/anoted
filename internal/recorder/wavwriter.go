package recorder

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"sync"
)

const wavHeaderSize = 44

// maxWAVDataSize is the largest data chunk a RIFF header can describe, since
// both size fields are uint32. Past this the sizes would silently wrap.
const maxWAVDataSize = int64(math.MaxUint32) - 36

// NewWAVWriter creates a WAV writer that streams s16le PCM straight to path.
//
// The file is opened up front with a placeholder header and PCM is appended as
// it arrives, so memory stays bounded for a multi-hour recording and an
// abnormal exit still leaves a file whose audio can be recovered — only the
// chunk sizes in the header are stale.
func NewWAVWriter(path string, sampleRate, channels int) (*WAVWriter, error) {
	if sampleRate <= 0 {
		sampleRate = 48000
	}
	if channels <= 0 {
		channels = 2
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, fmt.Errorf("create wav %s: %w", path, err)
	}
	if _, err := f.Write(minimalWAVHeader(sampleRate, channels, 0)); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("write wav header %s: %w", path, err)
	}
	return &WAVWriter{
		f:          f,
		buf:        bufio.NewWriterSize(f, 64*1024),
		sampleRate: sampleRate,
		channels:   channels,
	}, nil
}

// WAVWriter streams interleaved s16le PCM into a WAV file.
type WAVWriter struct {
	mu         sync.Mutex
	f          *os.File
	buf        *bufio.Writer
	sampleRate int
	channels   int
	n          int64
	err        error
	closed     bool
}

// WritePCM appends interleaved s16le samples. It is called from the audio
// callback, which has nowhere useful to report an error, so the first failure
// is retained and surfaced by Close instead.
func (w *WAVWriter) WritePCM(pcm []byte) {
	if len(pcm) == 0 {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed || w.err != nil {
		return
	}
	n, err := w.buf.Write(pcm)
	w.n += int64(n)
	if err != nil {
		w.err = fmt.Errorf("write pcm: %w", err)
	}
}

// Written reports how many PCM bytes have been accepted so far.
func (w *WAVWriter) Written() int64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.n
}

// Close flushes buffered PCM and patches the RIFF and data chunk sizes.
// It reports the first error seen during the whole recording, including
// deferred WritePCM failures.
func (w *WAVWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return w.err
	}
	w.closed = true

	if err := w.buf.Flush(); err != nil && w.err == nil {
		w.err = fmt.Errorf("flush pcm: %w", err)
	}
	size := w.n
	if size > maxWAVDataSize {
		if w.err == nil {
			w.err = fmt.Errorf("recording exceeds WAV size limit: %d bytes", size)
		}
		size = maxWAVDataSize
	}
	if _, err := w.f.WriteAt(minimalWAVHeader(w.sampleRate, w.channels, int(size)), 0); err != nil && w.err == nil {
		w.err = fmt.Errorf("patch wav header: %w", err)
	}
	if err := w.f.Close(); err != nil && w.err == nil {
		w.err = fmt.Errorf("close wav: %w", err)
	}
	return w.err
}

func minimalWAVHeader(sampleRate, channels, dataSize int) []byte {
	bitsPerSample := 16
	byteRate := sampleRate * channels * bitsPerSample / 8
	blockAlign := channels * bitsPerSample / 8
	fileSize := 36 + dataSize

	buf := make([]byte, wavHeaderSize)
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
