package transcribe

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Progress carries transcription progress updates for the UI.
type Progress struct {
	Percent       float64
	SegmentText   string
	AudioDuration time.Duration
}

// ProgressFunc receives progress updates during transcription.
type ProgressFunc func(Progress)

const maxPreviewLogLines = 30

// ParseCppProgressLine extracts a percentage from whisper.cpp stderr output.
// Example: "whisper_print_progress_callback: progress =  65%"
func ParseCppProgressLine(line string) (float64, bool) {
	const prefix = "progress ="
	idx := strings.Index(line, prefix)
	if idx < 0 {
		return 0, false
	}
	rest := strings.TrimSpace(line[idx+len(prefix):])
	rest = strings.TrimSuffix(rest, "%")
	v, err := strconv.ParseFloat(strings.TrimSpace(rest), 64)
	if err != nil {
		return 0, false
	}
	if v < 0 {
		v = 0
	}
	if v > 100 {
		v = 100
	}
	return v, true
}

// openai-whisper prints MM:SS.mmm, but switches to HH:MM:SS.mmm once a segment
// passes the one-hour mark, so the hour field has to be optional.
var segmentTimestampRE = regexp.MustCompile(`\[(?:(\d+):)?(\d{2}):(\d{2})\.(\d{3})\s*-->\s*(?:(\d+):)?(\d{2}):(\d{2})\.(\d{3})\]`)

// ParseSegmentLine extracts end timestamp and full line from openai-whisper verbose output.
func ParseSegmentLine(line string, audioDuration time.Duration) (percent float64, segment string, ok bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return 0, "", false
	}
	m := segmentTimestampRE.FindStringSubmatch(line)
	if m == nil {
		return 0, "", false
	}
	end := parseTimestamp(m[5], m[6], m[7], m[8])
	if end <= 0 {
		return 0, line, true
	}
	if audioDuration > 0 {
		percent = end.Seconds() / audioDuration.Seconds() * 100
		if percent > 100 {
			percent = 100
		}
	}
	return percent, line, true
}

// hour may be empty when whisper omits the field for sub-hour timestamps.
func parseTimestamp(hour, min, sec, ms string) time.Duration {
	h, _ := strconv.Atoi(hour)
	mi, _ := strconv.Atoi(min)
	s, _ := strconv.Atoi(sec)
	millis, _ := strconv.Atoi(ms)
	return time.Duration(h)*time.Hour + time.Duration(mi)*time.Minute +
		time.Duration(s)*time.Second + time.Duration(millis)*time.Millisecond
}

// ParseTqdmProgressLine extracts percentage from tqdm stderr output.
// Example: " 65%|██████▌   | 12345/67890 [00:30<00:15, 412.00frames/s]"
func ParseTqdmProgressLine(line string) (float64, bool) {
	line = strings.TrimSpace(line)
	if line == "" || !strings.Contains(line, "%") {
		return 0, false
	}
	pctIdx := strings.Index(line, "%")
	if pctIdx <= 0 {
		return 0, false
	}
	start := pctIdx - 1
	for start >= 0 && (line[start] >= '0' && line[start] <= '9' || line[start] == '.') {
		start--
	}
	num := strings.TrimSpace(line[start+1 : pctIdx])
	v, err := strconv.ParseFloat(num, 64)
	if err != nil {
		return 0, false
	}
	if v < 0 {
		v = 0
	}
	if v > 100 {
		v = 100
	}
	return v, true
}

// ComputeETA estimates remaining time from percent complete and elapsed wall time.
func ComputeETA(percent float64, elapsed time.Duration) time.Duration {
	if percent <= 5 || elapsed <= 0 {
		return 0
	}
	remaining := elapsed.Seconds() / (percent / 100) * (1 - percent/100)
	if remaining < 0 {
		return 0
	}
	return time.Duration(remaining * float64(time.Second))
}

// FormatETA formats a duration as "0m 15s".
func FormatETA(d time.Duration) string {
	if d <= 0 {
		return "0m 00s"
	}
	d = d.Round(time.Second)
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%dm %02ds", m, s)
}

func emitProgress(onProgress ProgressFunc, p Progress) {
	if onProgress == nil {
		return
	}
	onProgress(p)
}

func scanLines(r io.Reader, fn func(string)) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	sc.Split(scanLinesOrCR)
	for sc.Scan() {
		fn(sc.Text())
	}
	return sc.Err()
}

// scanLinesOrCR splits on \n and on \r.
//
// openai-whisper draws its tqdm progress bar with carriage returns and emits no
// newline until the bar closes, so a plain line splitter saw the whole bar as a
// single token. On a long CPU transcription that token grew past the 1 MiB
// limit and the scan failed — turning a run that had already written
// transcript.txt to disk into a reported failure. It also meant
// ParseTqdmProgressLine essentially never fired while the run was in progress,
// only once at the end, so the progress bar in the UI barely moved.
func scanLinesOrCR(data []byte, atEOF bool) (int, []byte, error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexAny(data, "\r\n"); i >= 0 {
		// Consume \r\n as one terminator so CRLF output does not yield a blank
		// token between every line.
		width := 1
		if data[i] == '\r' && i+1 < len(data) && data[i+1] == '\n' {
			width = 2
		}
		return i + width, data[:i], nil
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}
