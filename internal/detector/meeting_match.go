package detector

import (
	"strings"
	"time"
)

func meetingAppBinaries() map[string]bool {
	return map[string]bool{
		"chrome": true, "chromium": true, "firefox": true, "msedge": true, "brave": true,
		"teams": true, "ms-teams": true, "msteams": true,
	}
}

func isMeetingApp(binary, appName string) bool {
	b := strings.ToLower(strings.TrimSuffix(binary, ".exe"))
	if meetingAppBinaries()[b] {
		return true
	}
	lower := strings.ToLower(appName)
	return strings.Contains(lower, "firefox") ||
		strings.Contains(lower, "chrome") ||
		strings.Contains(lower, "chromium") ||
		strings.Contains(lower, "teams") ||
		strings.Contains(lower, "edge") ||
		strings.Contains(lower, "brave")
}

func matchMeetingText(mediaName, appName string, providers map[string][]string) (provider, title string) {
	for _, text := range []string{mediaName, appName} {
		if text == "" {
			continue
		}
		if p := MatchProvider(text, providers); p != ProviderUnknown {
			return p, text
		}
	}
	// PipeWire media.name from Firefox Meet tabs, e.g. "Meet - Daily standup".
	if mediaName != "" {
		lower := strings.ToLower(strings.TrimSpace(mediaName))
		if strings.HasPrefix(lower, "meet -") || strings.HasPrefix(lower, "meet |") {
			return ProviderGoogleMeet, mediaName
		}
	}
	return ProviderUnknown, ""
}

func snapshotFromMicCapture(c micCapture, providers map[string][]string) (Snapshot, bool) {
	if !isMeetingApp(c.Binary, c.AppName) {
		return Snapshot{}, false
	}
	provider, title := matchMeetingText(c.MediaName, c.AppName, providers)
	if provider == ProviderUnknown {
		return Snapshot{}, false
	}
	return meetingSnapshot(c.Binary, provider, title), true
}

// snapshotFromBrowserMicAndTitles matches an active browser mic capture against window titles
// when WASAPI session display names are generic (common on Windows).
func snapshotFromBrowserMicAndTitles(c micCapture, titles []string, providers map[string][]string) (Snapshot, bool) {
	if !isMeetingApp(c.Binary, c.AppName) {
		return Snapshot{}, false
	}
	for _, title := range titles {
		provider, matched := matchMeetingText(title, "", providers)
		if provider != ProviderUnknown {
			return meetingSnapshot(c.Binary, provider, matched), true
		}
	}
	return Snapshot{}, false
}

func meetingSnapshot(browser, provider, title string) Snapshot {
	return Snapshot{
		State: MeetingState{
			InMeeting: true,
			Provider:  provider,
			Title:     title,
			Browser:   browser,
		},
		CheckedAt: time.Now(),
	}
}
