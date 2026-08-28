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
//
// The longest matching pattern wins, with the provider name as a tiebreak. Two
// things made the naive version misbehave:
//
//   - Ranging over the map made the winner random when a title matched two
//     providers. The provider is part of the meeting session key, so it flipped
//     between polls, which read as a brand new meeting every two seconds and
//     re-raised the "start recording?" prompt the user had just dismissed.
//   - strings.Contains(x, "") is always true, so a single blank entry in the
//     patterns list — easy to produce by hand-editing the YAML — matched every
//     window title and, with auto-record on, recorded continuously.
func MatchProvider(text string, providers map[string][]string) string {
	lower := strings.ToLower(text)
	best, bestLen := ProviderUnknown, -1
	for name, patterns := range providers {
		for _, p := range patterns {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			if !strings.Contains(lower, strings.ToLower(p)) {
				continue
			}
			if len(p) > bestLen || (len(p) == bestLen && name < best) {
				best, bestLen = name, len(p)
			}
		}
	}
	return best
}
