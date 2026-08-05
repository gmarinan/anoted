package config

import "testing"

func TestApplyDefaultsSeedsUnconfiguredProviders(t *testing.T) {
	cfg := Default()
	cfg.Detection.Providers = map[string]ProviderConfig{
		"google_meet": {Patterns: []string{"meet.google.com"}},
	}
	cfg.applyDefaults()

	// A provider the user never configured still gets its built-in patterns.
	other := Default().Detection.Providers
	for name := range other {
		if name == "google_meet" {
			continue
		}
		if len(cfg.Detection.Providers[name].Patterns) == 0 {
			t.Fatalf("provider %q was not seeded with defaults", name)
		}
	}
}

func TestApplyDefaultsKeepsDeletedPatternsDeleted(t *testing.T) {
	// Re-adding built-in patterns to a provider the user has already configured
	// made deleting one in the Config tab a permanent no-op, and applyDefaults
	// runs on both Load and Save so hand-editing the YAML did not help either.
	cfg := Default()
	cfg.Detection.Providers = map[string]ProviderConfig{
		"google_meet": {Patterns: []string{"meet.google.com"}},
	}
	cfg.applyDefaults()

	got := cfg.Detection.Providers["google_meet"].Patterns
	if len(got) != 1 || got[0] != "meet.google.com" {
		t.Fatalf("user patterns were modified: got %v, want [meet.google.com]", got)
	}
}

func TestApplyDefaultsDoesNotMutateCallersMap(t *testing.T) {
	// Config is copied by value but the map header is shared, so mutating it
	// here raced View's reads on the Bubble Tea Update loop.
	providers := map[string]ProviderConfig{
		"google_meet": {Patterns: []string{"meet.google.com"}},
	}
	cfg := Default()
	cfg.Detection.Providers = providers

	cfg.applyDefaults()

	if len(providers) != 1 {
		t.Fatalf("caller's map gained entries: %v", providers)
	}
	if got := providers["google_meet"].Patterns; len(got) != 1 {
		t.Fatalf("caller's patterns were mutated: %v", got)
	}
}
