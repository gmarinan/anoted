package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"anoted/internal/config"
)

// MaxLogBytes is the size at which anoted.log is rotated.
//
// The log had no bound at all, and `anoted watch` is meant to run from login
// for weeks at a time, so the file grew until something else noticed. One
// rotation keeps the previous run's context without unbounded growth.
const MaxLogBytes = 4 << 20 // 4 MiB

// Setup configures structured logging to stderr and the log file.
// Use for CLI commands (doctor, status) where stderr is safe.
//
// The returned Closer owns the log file; callers must close it. Both
// constructors used to open the file and drop the handle, so it was never
// closed and the buffered tail of a run could be lost.
func Setup(level slog.Level) (*slog.Logger, io.Closer, error) {
	opts := &slog.HandlerOptions{Level: level}

	f, err := openLogFile()
	if err != nil {
		// Losing the file log is not fatal, but it used to be invisible — the
		// error was swallowed entirely, so a user chasing a bug had no idea why
		// the log they were told to read did not exist.
		fmt.Fprintf(os.Stderr, "anoted: file logging unavailable: %v\n", err)
		return slog.New(slog.NewJSONHandler(os.Stderr, opts)), io.NopCloser(nil), nil
	}

	multi := io.MultiWriter(os.Stderr, f)
	return slog.New(slog.NewJSONHandler(multi, opts)), f, nil
}

// SetupFile configures logging to the log file only (no stderr).
// Use for the TUI so slog output does not corrupt the AltScreen.
func SetupFile(level slog.Level) (*slog.Logger, io.Closer, error) {
	opts := &slog.HandlerOptions{Level: level}

	f, err := openLogFile()
	if err != nil {
		// stderr is unusable here: it would be painted over the alternate
		// screen. Fall back to discarding and report through the error.
		return slog.New(slog.NewJSONHandler(io.Discard, opts)), io.NopCloser(nil), err
	}
	return slog.New(slog.NewJSONHandler(f, opts)), f, nil
}

// ParseLevel maps a --log-level flag value onto a slog level.
//
// The level was hardcoded to Info, with no flag and no environment variable, so
// every slog.Debug call — including the auto-record decision trace and the
// detection poll, which is exactly what you need to diagnose a missed meeting —
// was unreachable in a real installation.
func ParseLevel(name string) (slog.Level, error) {
	switch name {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("unknown log level %q (debug, info, warn, error)", name)
	}
}

// Path returns the log file location so doctor and error messages can point at
// it. Users had no way to find out where to look.
func Path() (string, error) { return logFilePath() }

func openLogFile() (*os.File, error) {
	logPath, err := logFilePath()
	if err != nil {
		return nil, err
	}
	if err := rotateIfLarge(logPath); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open log %s: %w", logPath, err)
	}
	return f, nil
}

// rotateIfLarge moves an oversized log aside, keeping exactly one generation.
func rotateIfLarge(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat log %s: %w", path, err)
	}
	if info.Size() < MaxLogBytes {
		return nil
	}
	prev := path + ".1"
	if err := os.Remove(prev); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove old log %s: %w", prev, err)
	}
	if err := os.Rename(path, prev); err != nil {
		return fmt.Errorf("rotate log %s: %w", path, err)
	}
	return nil
}

func logFilePath() (string, error) {
	dir, err := config.ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "anoted.log"), nil
}
