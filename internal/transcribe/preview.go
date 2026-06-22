package transcribe

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ReadPreview returns the first maxLines non-empty lines from transcript.txt.
func ReadPreview(sessionDir string, maxLines int) (string, error) {
	if maxLines <= 0 {
		maxLines = 12
	}
	path := filepath.Join(sessionDir, TranscriptBaseName+".txt")
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var lines []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		lines = append(lines, line)
		if len(lines) >= maxLines {
			break
		}
	}
	if err := sc.Err(); err != nil {
		return "", fmt.Errorf("read transcript: %w", err)
	}
	if len(lines) == 0 {
		return "", nil
	}
	return strings.Join(lines, "\n"), nil
}
