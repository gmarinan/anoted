package transcribe

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"time"
)

// maxWAVChunkScan bounds how far into the file we look for the data chunk, so a
// malformed or hostile header cannot make this loop for a long time.
const maxWAVChunkScan = 64

// AudioDuration reads the duration of a PCM WAV file from its header.
//
// The chunk list is walked rather than read at fixed offsets: ffmpeg — which
// writes every recording on Linux — emits a LIST/INFO chunk before the data
// chunk, so the old hard-coded offsets read the LIST size as the sample count
// and reported a multi-hour meeting as a fraction of a second. That value feeds
// the transcription progress bar and the Obsidian note's duration field.
func AudioDuration(path string) (time.Duration, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("open wav: %w", err)
	}
	defer f.Close()

	var riff [12]byte
	if _, err := io.ReadFull(f, riff[:]); err != nil {
		return 0, fmt.Errorf("read wav header: %w", err)
	}
	if string(riff[0:4]) != "RIFF" || string(riff[8:12]) != "WAVE" {
		return 0, fmt.Errorf("not a WAV file")
	}

	var (
		channels, sampleRate, bitsPerSample int
		dataSize                            int64
		haveFmt, haveData                   bool
	)
	for i := 0; i < maxWAVChunkScan && !(haveFmt && haveData); i++ {
		var hdr [8]byte
		if _, err := io.ReadFull(f, hdr[:]); err != nil {
			break
		}
		id := string(hdr[0:4])
		size := int64(binary.LittleEndian.Uint32(hdr[4:8]))

		switch id {
		case "fmt ":
			body := make([]byte, 16)
			if size < 16 {
				return 0, fmt.Errorf("invalid wav fmt chunk")
			}
			if _, err := io.ReadFull(f, body); err != nil {
				return 0, fmt.Errorf("read wav fmt chunk: %w", err)
			}
			channels = int(binary.LittleEndian.Uint16(body[2:4]))
			sampleRate = int(binary.LittleEndian.Uint32(body[4:8]))
			bitsPerSample = int(binary.LittleEndian.Uint16(body[14:16]))
			haveFmt = true
			if _, err := f.Seek(size-16, io.SeekCurrent); err != nil {
				return 0, fmt.Errorf("seek wav fmt chunk: %w", err)
			}
		case "data":
			dataSize = size
			haveData = true
		default:
			if _, err := f.Seek(size, io.SeekCurrent); err != nil {
				break
			}
		}
		// Chunks are word-aligned; an odd size carries a pad byte.
		if size%2 == 1 {
			if _, err := f.Seek(1, io.SeekCurrent); err != nil {
				break
			}
		}
	}

	if !haveFmt || !haveData {
		return 0, fmt.Errorf("wav is missing fmt or data chunk")
	}
	if sampleRate <= 0 || channels <= 0 || bitsPerSample <= 0 {
		return 0, fmt.Errorf("invalid wav format")
	}
	bytesPerSample := bitsPerSample / 8
	if bytesPerSample <= 0 {
		return 0, fmt.Errorf("invalid bits per sample")
	}

	// A recording interrupted before its header was patched reports data size 0
	// even though PCM follows; fall back to what is actually on disk.
	if dataSize == 0 {
		if st, err := f.Stat(); err == nil {
			if remaining := st.Size() - 44; remaining > 0 {
				dataSize = remaining
			}
		}
	}

	samples := dataSize / int64(channels*bytesPerSample)
	secs := float64(samples) / float64(sampleRate)
	return time.Duration(secs * float64(time.Second)), nil
}
