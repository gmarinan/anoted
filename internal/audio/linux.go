//go:build linux

package audio

import (
	"fmt"
	"os/exec"
	"strings"
)

type linuxProvider struct{}

func newProvider() Provider { return linuxProvider{} }

func (linuxProvider) List() (Catalog, error) {
	defaultSink, _ := pactlGet("get-default-sink")
	defaultMonitor, _ := defaultMonitorSource()
	defaultMic, _ := defaultMicSource()
	linked := listSinkLinkedApps()

	sinks, err := listSinks()
	if err != nil {
		return Catalog{}, err
	}

	sources, err := listCaptureSources()
	if err != nil {
		return Catalog{}, err
	}

	var cat Catalog
	cat.Outputs = append(cat.Outputs, Device{
		ID: AutoID, Name: AutoLabel, IsDefault: true,
	})
	cat.Microphones = append(cat.Microphones, Device{
		ID: AutoID, Name: AutoLabel, IsDefault: true,
	})

	for _, s := range sinks {
		monitorID := s.name + ".monitor"
		cat.Outputs = append(cat.Outputs, Device{
			ID:         monitorID,
			Name:       s.name,
			State:      s.state,
			NodeID:     s.index,
			Format:     s.format,
			IsDefault:  s.name == defaultSink,
			LinkedApps: linked[s.index],
		})
		_ = defaultMonitor // used implicitly via IsDefault on matching sink
	}

	for _, s := range sources {
		cat.Microphones = append(cat.Microphones, Device{
			ID:        s.name,
			Name:      s.name,
			State:     s.state,
			NodeID:    s.index,
			Format:    s.format,
			IsDefault: s.name == defaultMic,
		})
	}
	return cat, nil
}

func (linuxProvider) Resolve(systemMonitor, microphone string) (string, string, error) {
	monitor := systemMonitor
	if monitor == "" {
		var err error
		monitor, err = defaultMonitorSource()
		if err != nil {
			return "", "", fmt.Errorf("resolve system monitor: %w", err)
		}
	}
	mic := microphone
	if mic == "" {
		var err error
		mic, err = defaultMicSource()
		if err != nil {
			return "", "", fmt.Errorf("resolve microphone: %w", err)
		}
	}
	return monitor, mic, nil
}

func (linuxProvider) MonitorWarning(configuredMonitor string) string {
	if configuredMonitor == "" {
		return ""
	}
	defaultMonitor, err := defaultMonitorSource()
	if err != nil {
		return ""
	}
	if configuredMonitor == defaultMonitor {
		return ""
	}
	sink := strings.TrimSuffix(configuredMonitor, ".monitor")
	state := sinkState(sink)
	defaultSink, _ := pactlGet("get-default-sink")
	if state == "SUSPENDED" || state == "IDLE" {
		return fmt.Sprintf(
			"Output %q is %s; audio likely plays on %q — use (auto) or pick the RUNNING sink",
			sink, state, defaultSink,
		)
	}
	return fmt.Sprintf(
		"Output %q is not the default (%q); system audio in the recording may be silent",
		sink, defaultSink,
	)
}

type nodeEntry struct {
	index  string
	name   string
	driver string
	format string
	state  string
}

func listSinks() ([]nodeEntry, error) {
	return listNodes("sinks")
}

func listCaptureSources() ([]nodeEntry, error) {
	all, err := listNodes("sources")
	if err != nil {
		return nil, err
	}
	var out []nodeEntry
	for _, s := range all {
		if strings.HasSuffix(s.name, ".monitor") {
			continue
		}
		if strings.HasPrefix(s.name, "v4l2_") {
			continue
		}
		out = append(out, s)
	}
	return out, nil
}

func listNodes(kind string) ([]nodeEntry, error) {
	path, err := exec.LookPath("pactl")
	if err != nil {
		return nil, fmt.Errorf("pactl not found: %w", err)
	}
	out, err := exec.Command(path, "list", kind, "short").Output()
	if err != nil {
		return nil, fmt.Errorf("pactl list %s: %w", kind, err)
	}
	var entries []nodeEntry
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		entry := nodeEntry{
			index: fields[0],
			name:  fields[1],
		}
		if len(fields) >= 3 {
			entry.driver = fields[2]
		}
		if len(fields) >= 4 {
			entry.format = fields[3]
		}
		if len(fields) >= 5 {
			entry.state = fields[4]
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// listSinkLinkedApps maps sink index -> application names currently playing audio.
func listSinkLinkedApps() map[string][]string {
	path, err := exec.LookPath("pactl")
	if err != nil {
		return nil
	}
	out, err := exec.Command(path, "list", "sink-inputs").Output()
	if err != nil {
		return nil
	}

	linked := make(map[string][]string)
	var sinkIdx string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Sink Input #") {
			sinkIdx = ""
			continue
		}
		if strings.HasPrefix(line, "Sink:") {
			sinkIdx = strings.TrimSpace(strings.TrimPrefix(line, "Sink:"))
			continue
		}
		if strings.HasPrefix(line, "application.name = ") && sinkIdx != "" {
			app := strings.Trim(strings.TrimPrefix(line, "application.name = "), "\"")
			if app != "" && !contains(linked[sinkIdx], app) {
				linked[sinkIdx] = append(linked[sinkIdx], app)
			}
		}
	}
	return linked
}

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

func sinkState(sinkName string) string {
	sinks, err := listSinks()
	if err != nil {
		return "unknown"
	}
	for _, s := range sinks {
		if s.name == sinkName {
			return s.state
		}
	}
	return "unknown"
}

func defaultMonitorSource() (string, error) {
	sink, err := pactlGet("get-default-sink")
	if err != nil {
		return "", err
	}
	return sink + ".monitor", nil
}

func defaultMicSource() (string, error) {
	return pactlGet("get-default-source")
}

func pactlGet(subcommand string) (string, error) {
	path, err := exec.LookPath("pactl")
	if err != nil {
		return "", fmt.Errorf("pactl not found: %w", err)
	}
	out, err := exec.Command(path, subcommand).Output()
	if err != nil {
		return "", fmt.Errorf("pactl %s: %w", subcommand, err)
	}
	name := strings.TrimSpace(string(out))
	if name == "" {
		return "", fmt.Errorf("pactl %s returned empty name", subcommand)
	}
	return name, nil
}
