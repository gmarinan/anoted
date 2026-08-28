package recorder

import (
	"context"
	"time"

	"anoted/internal/session"
)

// Status describes recorder state.
type Status string

const (
	StatusIdle      Status = "idle"
	StatusRecording Status = "recording"
	StatusStopping  Status = "stopping"
	StatusError     Status = "error"
)

// RecorderStatus is a point-in-time recorder snapshot.
type RecorderStatus struct {
	Status     Status
	Backend    string
	SessionDir string
	StartedAt  time.Time
	Error      string
}

// SessionConfig configures a recording session.
type SessionConfig struct {
	OutputRoot    string
	Provider      session.Provider
	Platform      string
	AutoRecord    bool
	Manual        bool
	SampleRate    int
	Channels      int
	SystemMonitor string
	Microphone    string
	OnSystemPCM   func([]byte)
	OnMicPCM      func([]byte)
}

// Recorder captures system and microphone audio.
type Recorder interface {
	Start(ctx context.Context, cfg SessionConfig) error
	Stop(ctx context.Context) error
	Status() RecorderStatus
	Name() string

	// Unusable explains why this backend cannot actually capture audio, or
	// returns "" when it can.
	//
	// Without this, a machine with no working backend silently fell back to
	// DummyRecorder: the UI showed RECORDING with a running timer, the tray
	// went red, a row was written to the database, and the result was a
	// 44-byte WAV. `anoted doctor` reported it as ok. Callers must refuse to
	// start, and say why, rather than pretend to record.
	Unusable() string
}
