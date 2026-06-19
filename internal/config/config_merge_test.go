package config

import "testing"

func TestMergeProviderPatterns(t *testing.T) {
	cfg := Default()
	cfg.Detection.Providers = map[string]ProviderConfig{
		"google_meet": {Patterns: []string{"meet.google.com", "Google Meet"}},
	}
	cfg.applyDefaults()

	patterns := cfg.Detection.Providers["google_meet"].Patterns
	hasMeetDash := false
	for _, p := range patterns {
		if p == "Meet -" {
			hasMeetDash = true
		}
	}
	if !hasMeetDash {
		t.Fatalf("expected Meet - pattern merged, got %v", patterns)
	}
}
