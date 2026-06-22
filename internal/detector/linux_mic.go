//go:build linux

package detector

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

// micCapture is an app actively recording from the microphone via PipeWire/PulseAudio.
type micCapture struct {
	Binary    string
	AppName   string
	MediaName string
}

func (d *linuxDetector) Poll(ctx context.Context) (Snapshot, error) {
	mode := d.cfg.Mode
	if mode == "" {
		mode = ModeMic
	}

	switch mode {
	case ModeNone:
		return Snapshot{State: MeetingState{}, CheckedAt: time.Now()}, nil
	case ModeWindow:
		return d.pollWindow(ctx)
	case ModeBoth:
		if snap, found := d.pollMic(ctx); found {
			return snap, nil
		}
		return d.pollWindow(ctx)
	default:
		return d.pollMicOnly(ctx)
	}
}

func (d *linuxDetector) pollMicOnly(ctx context.Context) (Snapshot, error) {
	snap, found := d.pollMic(ctx)
	if found {
		return snap, nil
	}
	return Snapshot{State: MeetingState{Warning: d.platformWarning()}, CheckedAt: time.Now()}, nil
}

func (d *linuxDetector) pollMic(ctx context.Context) (Snapshot, bool) {
	captures, err := listMicCaptures(ctx)
	if err != nil {
		return Snapshot{State: MeetingState{Warning: err.Error()}, CheckedAt: time.Now()}, false
	}

	for _, c := range captures {
		if !isMeetingApp(c.Binary, c.AppName) {
			continue
		}
		provider, title := matchMeetingText(c.MediaName, c.AppName, d.cfg.Providers)
		if provider == ProviderUnknown && isTeamsBinary(c.Binary) {
			provider = ProviderTeams
			title = c.MediaName
		}
		if provider != ProviderUnknown {
			return Snapshot{
				State: MeetingState{
					InMeeting: true,
					Provider:  provider,
					Title:     title,
					Browser:   c.Binary,
					Warning:   d.platformWarning(),
				},
				CheckedAt: time.Now(),
			}, true
		}
	}

	return Snapshot{}, false
}

func (d *linuxDetector) pollWindow(ctx context.Context) (Snapshot, error) {
	state := MeetingState{Warning: d.platformWarning()}

	procs, err := listProcesses(ctx)
	if err != nil {
		return Snapshot{State: state, CheckedAt: time.Now()}, err
	}

	browserProcs := meetingAppBinaries()
	var titles []string
	if d.plat.Session == "x11" && d.cfg.WindowTool != "none" {
		titles = d.windowTitles(ctx)
	}

	for _, p := range procs {
		base := strings.ToLower(p)
		if !browserProcs[base] {
			continue
		}
		for _, title := range titles {
			if provider := MatchProvider(title, d.cfg.Providers); provider != ProviderUnknown {
				state.InMeeting = true
				state.Provider = provider
				state.Title = title
				state.Browser = base
				return Snapshot{State: state, CheckedAt: time.Now()}, nil
			}
		}
		if len(titles) == 0 && browserProcs[base] {
			if state.Warning == "" {
				state.Warning = "Browser running but meeting not confirmed from window titles"
			}
		}
	}

	return Snapshot{State: state, CheckedAt: time.Now()}, nil
}

func (d *linuxDetector) platformWarning() string {
	if d.plat.Session == "wayland" && d.cfg.Mode == ModeWindow {
		return "Wayland: window titles may be unavailable; consider detection.mode: mic"
	}
	return ""
}

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

func isTeamsBinary(binary string) bool {
	b := strings.ToLower(strings.TrimSuffix(binary, ".exe"))
	return b == "teams" || b == "ms-teams" || b == "msteams"
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

func listMicCaptures(ctx context.Context) ([]micCapture, error) {
	path, err := exec.LookPath("pactl")
	if err != nil {
		return nil, err
	}
	out, err := exec.CommandContext(ctx, path, "list", "source-outputs").Output()
	if err != nil {
		return nil, err
	}

	var captures []micCapture
	var cur *micCapture
	flush := func() {
		if cur == nil {
			return
		}
		if cur.Binary != "" || cur.AppName != "" {
			captures = append(captures, *cur)
		}
		cur = nil
	}

	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Source Output #") {
			flush()
			cur = &micCapture{}
			continue
		}
		if cur == nil {
			continue
		}
		switch {
		case strings.HasPrefix(line, "application.process.binary = "):
			cur.Binary = unquoteProp(line)
		case strings.HasPrefix(line, "application.name = "):
			cur.AppName = unquoteProp(line)
		case strings.HasPrefix(line, "media.name = "):
			cur.MediaName = unquoteProp(line)
		}
	}
	flush()
	return captures, nil
}

func unquoteProp(line string) string {
	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return ""
	}
	return strings.Trim(strings.TrimSpace(parts[1]), "\"")
}
