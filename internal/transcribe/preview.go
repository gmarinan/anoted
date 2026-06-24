package transcribe

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"anoted/internal/config"
)

// ReadPreview returns the first maxLines non-empty lines from transcript output for a session.
func ReadPreview(sessionDir string, tcfg config.TranscriptionConfig, maxLines int) (string, error) {
	if maxLines <= 0 {
		maxLines = 12
	}
	dir, err := OutputDir(tcfg, sessionDir)
	if err != nil {
		dir = sessionDir
	}
	fileBase := outputFileBase(tcfg, sessionDir)
	path := filepath.Join(dir, fileBase+".txt")
	text, err := readPreviewFile(path, maxLines)
	if err == nil && text != "" {
		return text, nil
	}
	mdPath := filepath.Join(dir, markdownFilename(tcfg, sessionDir))
	body, mdErr := ExtractMarkdownBody(mdPath)
	if mdErr != nil {
		if err != nil {
			return "", err
		}
		return "", mdErr
	}
	return firstNLines(body, maxLines), nil
}

func readPreviewFile(path string, maxLines int) (string, error) {
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

func firstNLines(text string, maxLines int) string {
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lines = append(lines, line)
		if len(lines) >= maxLines {
			break
		}
	}
	return strings.Join(lines, "\n")
}
