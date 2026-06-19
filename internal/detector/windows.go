//go:build windows

package detector

import (
	"context"
	"os/exec"
	"strings"
	"time"

	"meetctl/internal/platform"
)

func newPlatformDetector(cfg Config, _ platform.Info) Detector {
	return &windowsDetector{cfg: cfg}
}

type windowsDetector struct {
	cfg Config
}

func (d *windowsDetector) Name() string { return "windows" }

func (d *windowsDetector) Poll(ctx context.Context) (Snapshot, error) {
	state := MeetingState{}

	procs, err := listProcesses(ctx)
	if err != nil {
		return Snapshot{State: state, CheckedAt: time.Now()}, err
	}

	titles := d.windowTitles(ctx)

	teamsProcs := map[string]bool{
		"ms-teams": true, "teams": true, "msteams": true,
	}
	browserProcs := map[string]bool{
		"chrome": true, "msedge": true, "firefox": true, "brave": true,
	}

	for _, p := range procs {
		base := strings.ToLower(strings.TrimSuffix(p, ".exe"))
		if teamsProcs[base] {
			state.InMeeting = true
			state.Provider = ProviderTeams
			state.Browser = base
			break
		}
	}

	for _, title := range titles {
		if provider := MatchProvider(title, d.cfg.Providers); provider != ProviderUnknown {
			state.InMeeting = true
			state.Provider = provider
			state.Title = title
			if state.Browser == "" {
				state.Browser = "browser"
			}
			break
		}
	}

	_ = browserProcs // reserved for future per-process title correlation

	return Snapshot{State: state, CheckedAt: time.Now()}, nil
}

func (d *windowsDetector) windowTitles(ctx context.Context) []string {
	script := `Get-Process | Where-Object { $_.MainWindowTitle } | ForEach-Object { $_.MainWindowTitle }`
	out, err := exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command", script).Output()
	if err != nil {
		return nil
	}
	var titles []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			titles = append(titles, line)
		}
	}
	return titles
}

func listProcesses(ctx context.Context) ([]string, error) {
	out, err := exec.CommandContext(ctx, "tasklist", "/FO", "CSV", "/NH").Output()
	if err != nil {
		return nil, err
	}
	var names []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if i := strings.Index(line, "\""); i >= 0 {
			rest := line[i+1:]
			if j := strings.Index(rest, "\""); j >= 0 {
				names = append(names, rest[:j])
			}
		}
	}
	return names, nil
}
