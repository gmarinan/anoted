package detector

import "testing"

// A blank pattern is easy to produce by hand-editing the YAML ("- " on its own
// line). strings.Contains(x, "") is always true, so it matched every window
// title — and with auto-record enabled that means recording continuously.
func TestMatchProviderIgnoresEmptyPatterns(t *testing.T) {
	providers := map[string][]string{
		"teams": {"", "   ", "teams.microsoft.com"},
	}
	if got := MatchProvider("reading the news", providers); got != ProviderUnknown {
		t.Fatalf("blank pattern matched an unrelated title: got %q", got)
	}
	if got := MatchProvider("Chat | teams.microsoft.com", providers); got != "teams" {
		t.Fatalf("real pattern stopped matching: got %q", got)
	}
}

// The provider is part of the meeting session key. Ranging over the map made
// the winner random when a title matched two providers, so the key flipped
// between polls, every poll looked like a brand new meeting, and the dismissed
// "start recording?" prompt came back every two seconds.
func TestMatchProviderIsDeterministicAcrossOverlaps(t *testing.T) {
	providers := map[string][]string{
		"google_meet": {"Meet -"},
		"teams":       {"Meet"},
	}
	const title = "Weekly sync | Meet - Google Chrome"

	first := MatchProvider(title, providers)
	for i := 0; i < 200; i++ {
		if got := MatchProvider(title, providers); got != first {
			t.Fatalf("provider flipped between calls: %q then %q", first, got)
		}
	}
	// The longer, more specific pattern should win.
	if first != "google_meet" {
		t.Fatalf("most specific pattern lost: got %q, want google_meet", first)
	}
}

func TestMatchProviderTiebreaksOnName(t *testing.T) {
	providers := map[string][]string{
		"zebra": {"call"},
		"alpha": {"call"},
	}
	for i := 0; i < 50; i++ {
		if got := MatchProvider("joining a call", providers); got != "alpha" {
			t.Fatalf("equal-length patterns must tiebreak deterministically: got %q", got)
		}
	}
}
