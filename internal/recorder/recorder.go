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
}

// Recorder captures system and microphone audio.
type Recorder interface {
	Start(ctx context.Context, cfg SessionConfig) error
	Stop(ctx context.Context) error
	Status() RecorderStatus
	Name() string
}
