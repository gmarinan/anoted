package detector

import "testing"

func TestMatchProvider(t *testing.T) {
	patterns := map[string][]string{
		"google_meet": {"meet.google.com", "Google Meet"},
		"teams":       {"teams.microsoft.com", "Meeting with", "In a call"},
	}
	if got := MatchProvider("Join - Google Meet", patterns); got != "google_meet" {
		t.Fatalf("got %q", got)
	}
	if got := MatchProvider("Meeting with Bob | Microsoft Teams", patterns); got != "teams" {
		t.Fatalf("got %q", got)
	}
	if got := MatchProvider("Chat | Microsoft Teams", patterns); got != ProviderUnknown {
		t.Fatalf("got %q", got)
	}
	if got := MatchProvider("Random Tab", patterns); got != ProviderUnknown {
		t.Fatalf("got %q", got)
	}
}

func TestMockDetector(t *testing.T) {
	d := NewMockDetector()
	d.InMeeting = true
	d.Provider = ProviderTeams
	snap, err := d.Poll(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if !snap.State.InMeeting || snap.State.Provider != ProviderTeams {
		t.Fatalf("unexpected state: %+v", snap.State)
	}
}
