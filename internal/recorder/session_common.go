package recorder

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"anoted/internal/session"
)

// SessionAudioFile is the mixed system + microphone recording filename.
const SessionAudioFile = "recording.wav"

// Recordings are readable only by the user who made them: anoted's privacy
// promise covers the people in the room, not just the operator, so meeting
// audio must not be world-readable on a shared machine. Defined in
// internal/session so there is one definition rather than two that can drift.
const (
	SessionDirMode  = session.SessionDirMode
	SessionFileMode = session.SessionFileMode
)

// maxSessionDirAttempts bounds the collision retry loop. Hitting it means
// something is badly wrong with the output directory, not that a user recorded
// 100 sessions in the same second.
const maxSessionDirAttempts = 100

// createSessionDir makes a fresh, exclusively-owned directory for one recording
// and returns its path.
//
// The name is second-resolution, so stopping and immediately restarting a
// recording produced the same path twice. Nothing detected that: MkdirAll
// succeeds on an existing directory, ffmpeg runs with -y and the Windows writer
// opens with O_TRUNC, so the second take silently overwrote the first — and both
// SQLite rows then pointed at one directory, making a delete of either destroy
// the other's audio. os.Mkdir fails on an existing directory, which turns the
// collision into a suffix instead of data loss.
func createSessionDir(sess SessionConfig) (string, error) {
	base := fmt.Sprintf("%s_%s", time.Now().Format("2006-01-02_15-04-05"), sess.Provider)
	if err := os.MkdirAll(sess.OutputRoot, SessionDirMode); err != nil {
		return "", fmt.Errorf("create output root %s: %w", sess.OutputRoot, err)
	}
	for attempt := 1; attempt <= maxSessionDirAttempts; attempt++ {
		name := base
		if attempt > 1 {
			name = fmt.Sprintf("%s_%d", base, attempt)
		}
		dir := filepath.Join(sess.OutputRoot, name)
		err := os.Mkdir(dir, SessionDirMode)
		if err == nil {
			return dir, nil
		}
		if !os.IsExist(err) {
			return "", fmt.Errorf("create session dir %s: %w", dir, err)
		}
	}
	return "", fmt.Errorf("create session dir: %s already exists with %d suffixes", base, maxSessionDirAttempts)
}

func dirFile(dir, name string) string {
	return filepath.Join(dir, name)
}
