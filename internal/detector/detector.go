package detector

import (
	"context"
	"time"
)

// MeetingState describes whether a meeting is active.
type MeetingState struct {
	InMeeting bool
	Provider  string
	Title     string
	Browser   string
	Warning   string
}

// Snapshot is a point-in-time detection result.
type Snapshot struct {
	State     MeetingState
	CheckedAt time.Time
}

// Detector polls for active meetings.
type Detector interface {
	Poll(ctx context.Context) (Snapshot, error)
	Name() string
}

// Config carries detection settings.
type Config struct {
	PollInterval time.Duration
	Providers    map[string][]string
	Mode         string
	WindowTool   string
}
