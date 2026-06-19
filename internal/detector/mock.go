package detector

import (
	"context"
	"strings"
	"time"
)

// MockDetector simulates meeting detection for development and tests.
type MockDetector struct {
	InMeeting bool
	Provider  string
	Title     string
}

func NewMockDetector() *MockDetector {
	return &MockDetector{
		Provider: string(ProviderUnknown),
		Title:    "",
	}
}

func (d *MockDetector) Name() string { return "mock" }

func (d *MockDetector) Poll(_ context.Context) (Snapshot, error) {
	return Snapshot{
		State: MeetingState{
			InMeeting: d.InMeeting,
			Provider:  d.Provider,
			Title:     d.Title,
		},
		CheckedAt: time.Now(),
	}, nil
}

// Provider constants used by detectors.
const (
	ProviderGoogleMeet = "google_meet"
	ProviderTeams      = "teams"
	ProviderUnknown    = "unknown"
)

// MatchProvider returns the provider key if text matches any configured pattern.
func MatchProvider(text string, providers map[string][]string) string {
	lower := strings.ToLower(text)
	for name, patterns := range providers {
		for _, p := range patterns {
			if strings.Contains(lower, strings.ToLower(p)) {
				return name
			}
		}
	}
	return ProviderUnknown
}
