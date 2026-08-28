package transcribe

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"anoted/internal/config"
)

// OutputDir returns the directory where transcript files are stored for a session.
// Empty transcription.output_dir uses the recording session folder.
func OutputDir(tcfg config.TranscriptionConfig, sessionDir string) (string, error) {
	raw := strings.TrimSpace(tcfg.OutputDir)
	if raw == "" {
		return sessionDir, nil
	}
	base, err := config.ExpandPath(raw)
	if err != nil {
		return "", fmt.Errorf("expand transcription output_dir: %w", err)
	}
	if err := os.MkdirAll(base, 0o700); err != nil {
		return "", fmt.Errorf("create transcript dir %s: %w", base, err)
	}
	return base, nil
}

// outputFileBase returns the basename for transcript.* files (without extension).
func outputFileBase(tcfg config.TranscriptionConfig, sessionDir string) string {
	if strings.TrimSpace(tcfg.OutputDir) != "" {
		name := filepath.Base(sessionDir)
		if name != "" && name != "." {
			return name
		}
	}
	return TranscriptBaseName
}

// markdownFilename returns the markdown output filename for a session.
func markdownFilename(tcfg config.TranscriptionConfig, sessionDir string) string {
	if strings.TrimSpace(tcfg.OutputDir) != "" {
		return outputFileBase(tcfg, sessionDir) + ".md"
	}
	name := strings.TrimSpace(tcfg.Markdown.Filename)
	if name == "" {
		return TranscriptBaseName + ".md"
	}
	return name
}
