package logging

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"anoted/internal/config"
)

// Setup configures structured logging to stderr and optionally a log file.
// Use for CLI commands (doctor, status) where stderr is safe.
func Setup(level slog.Level) (*slog.Logger, error) {
	opts := &slog.HandlerOptions{Level: level}
	handler := slog.NewJSONHandler(os.Stderr, opts)

	logPath, err := logFilePath()
	if err != nil {
		return slog.New(handler), nil
	}

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return slog.New(handler), nil
	}

	multi := io.MultiWriter(os.Stderr, f)
	return slog.New(slog.NewJSONHandler(multi, opts)), nil
}

// SetupFile configures logging to the log file only (no stderr).
// Use for the TUI so slog output does not corrupt the AltScreen.
func SetupFile(level slog.Level) (*slog.Logger, error) {
	opts := &slog.HandlerOptions{Level: level}
	logPath, err := logFilePath()
	if err != nil {
		return slog.New(slog.NewJSONHandler(io.Discard, opts)), nil
	}
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return slog.New(slog.NewJSONHandler(io.Discard, opts)), nil
	}
	return slog.New(slog.NewJSONHandler(f, opts)), nil
}

func logFilePath() (string, error) {
	dir, err := config.ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "anoted.log"), nil
}
