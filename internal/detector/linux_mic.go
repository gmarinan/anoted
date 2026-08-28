//go:build linux

package detector

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

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
	// Keep any warning pollMic produced. Dropping it meant that once pactl
	// started failing (PipeWire restart moving the pulse socket, say) detection
	// silently reported "idle" forever with nothing on screen to explain why.
	warning := snap.State.Warning
	if warning == "" {
		warning = d.platformWarning()
	}
	return Snapshot{State: MeetingState{Warning: warning}, CheckedAt: time.Now()}, nil
}

func (d *linuxDetector) pollMic(ctx context.Context) (Snapshot, bool) {
	captures, err := listMicCaptures(ctx)
	if err != nil {
		return Snapshot{State: MeetingState{Warning: err.Error()}, CheckedAt: time.Now()}, false
	}

	for _, c := range captures {
		if snap, ok := snapshotFromMicCapture(c, d.cfg.Providers); ok {
			snap.State.Warning = d.platformWarning()
			return snap, true
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

// pactlPath is resolved once: this runs on every detection poll for the life of
// the process, and $PATH does not change underneath a running anoted.
var pactlPath = sync.OnceValues(func() (string, error) {
	return exec.LookPath("pactl")
})

func listMicCaptures(ctx context.Context) ([]micCapture, error) {
	path, err := pactlPath()
	if err != nil {
		return nil, fmt.Errorf("find pactl: %w", err)
	}
	out, err := exec.CommandContext(ctx, path, "list", "source-outputs").Output()
	if err != nil {
		return nil, fmt.Errorf("pactl list source-outputs: %w", err)
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
