package transcribe

import (
	"encoding/binary"
	"fmt"
	"os"
	"time"
)

// AudioDuration reads the duration of a PCM WAV file from its header.
func AudioDuration(path string) (time.Duration, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("open wav: %w", err)
	}
	defer f.Close()

	hdr := make([]byte, 44)
	if _, err := f.Read(hdr); err != nil {
		return 0, fmt.Errorf("read wav header: %w", err)
	}
	if string(hdr[0:4]) != "RIFF" || string(hdr[8:12]) != "WAVE" {
		return 0, fmt.Errorf("not a WAV file")
	}

	channels := int(binary.LittleEndian.Uint16(hdr[22:24]))
	sampleRate := int(binary.LittleEndian.Uint32(hdr[24:28]))
	bitsPerSample := int(binary.LittleEndian.Uint16(hdr[34:36]))
	dataSize := int(binary.LittleEndian.Uint32(hdr[40:44]))

	if sampleRate <= 0 || channels <= 0 || bitsPerSample <= 0 {
		return 0, fmt.Errorf("invalid wav format")
	}
	bytesPerSample := bitsPerSample / 8
	if bytesPerSample <= 0 {
		return 0, fmt.Errorf("invalid bits per sample")
	}

	samples := dataSize / (channels * bytesPerSample)
	secs := float64(samples) / float64(sampleRate)
	return time.Duration(secs * float64(time.Second)), nil
}
