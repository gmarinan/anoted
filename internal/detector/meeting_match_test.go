package detector

import "testing"

func TestMatchMeetingTextMediaName(t *testing.T) {
	patterns := map[string][]string{
		"google_meet": {"meet.google.com", "Google Meet"},
	}
	provider, title := matchMeetingText("Meet - Daily Innovación", "Firefox", patterns)
	if provider != ProviderGoogleMeet {
		t.Fatalf("got %q title %q", provider, title)
	}
}

func TestMatchMeetingTextWithDefaultPatterns(t *testing.T) {
	patterns := map[string][]string{
		"google_meet": {"meet.google.com", "Google Meet", "Meet -", "Meet |"},
	}
	provider, _ := matchMeetingText("Meet - Daily Innovación", "Firefox", patterns)
	if provider != ProviderGoogleMeet {
		t.Fatalf("got %q", provider)
	}
}

func TestIsMeetingApp(t *testing.T) {
	if !isMeetingApp("firefox", "Firefox") {
		t.Fatal("firefox should match")
	}
	if isMeetingApp("obs", "OBS") {
		t.Fatal("obs should not match")
	}
}

func TestSnapshotFromMicCaptureTeams(t *testing.T) {
	patterns := map[string][]string{
		"teams": {"Meeting with", "In a call"},
	}
	c := micCapture{
		Binary:    "ms-teams",
		AppName:   "Microsoft Teams",
		MediaName: "Meeting with Alice",
	}
	snap, ok := snapshotFromMicCapture(c, patterns)
	if !ok || !snap.State.InMeeting || snap.State.Provider != ProviderTeams {
		t.Fatalf("unexpected: ok=%v snap=%+v", ok, snap.State)
	}
}

func TestSnapshotFromMicCaptureIdleTeamsChat(t *testing.T) {
	patterns := map[string][]string{
		"teams": {"teams.microsoft.com", "Meeting with", "| Meet", "In a call"},
	}
	c := micCapture{
		Binary:    "firefox",
		AppName:   "Microsoft Teams",
		MediaName: "Chat | Microsoft Teams",
	}
	_, ok := snapshotFromMicCapture(c, patterns)
	if ok {
		t.Fatal("idle Teams chat title should not match tightened patterns")
	}
}

func TestSnapshotFromMicCaptureTeamsLobbyNoBinaryFallback(t *testing.T) {
	patterns := map[string][]string{
		"teams": {"Meeting with", "In a call"},
	}
	c := micCapture{
		Binary:    "ms-teams",
		AppName:   "Communications",
		MediaName: "",
	}
	_, ok := snapshotFromMicCapture(c, patterns)
	if ok {
		t.Fatal("teams mic alone in lobby should not match without call-specific title")
	}
}

func TestSnapshotFromBrowserMicAndTitlesFirefoxMeet(t *testing.T) {
	patterns := map[string][]string{
		"google_meet": {"meet.google.com", "Google Meet", "Meet -", "Meet |"},
	}
	c := micCapture{
		Binary:    "firefox",
		AppName:   "Firefox",
		MediaName: "Firefox",
	}
	titles := []string{"Meet - Seguimiento Soporte Automatización — Mozilla Firefox"}
	snap, ok := snapshotFromBrowserMicAndTitles(c, titles, patterns)
	if !ok || snap.State.Provider != ProviderGoogleMeet {
		t.Fatalf("expected google_meet: ok=%v %+v", ok, snap.State)
	}
}

func TestSnapshotFromBrowserMicAndTitlesGmailNoMatch(t *testing.T) {
	patterns := map[string][]string{
		"google_meet": {"meet.google.com", "Google Meet", "Meet -", "Meet |"},
	}
	c := micCapture{
		Binary:    "firefox",
		AppName:   "Firefox",
		MediaName: "Firefox",
	}
	titles := []string{"Recibidos (1) - gmarinan@tech… — Mozilla Firefox"}
	_, ok := snapshotFromBrowserMicAndTitles(c, titles, patterns)
	if ok {
		t.Fatal("gmail tab with mic should not match meet patterns")
	}
}

func TestSnapshotFromBrowserMicAndTitlesTeamsCall(t *testing.T) {
	patterns := map[string][]string{
		"teams": {"Meeting with", "In a call"},
	}
	c := micCapture{
		Binary:    "ms-teams",
		AppName:   "Communications",
		MediaName: "",
	}
	titles := []string{"Meeting with Bob | Microsoft Teams"}
	snap, ok := snapshotFromBrowserMicAndTitles(c, titles, patterns)
	if !ok || snap.State.Provider != ProviderTeams {
		t.Fatalf("expected teams from window title: ok=%v %+v", ok, snap.State)
	}
}
