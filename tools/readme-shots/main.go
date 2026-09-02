// Command readme-shots prints a static TUI frame for README screenshots.
//
//	go run ./tools/readme-shots recording | freeze -c docs/assets/freeze.json -o docs/assets/home-recording.png
package main

import (
	"os"
	"time"

	"anoted/internal/doctor"
	"anoted/internal/platform"
	"anoted/internal/session"
	"anoted/internal/tui/components"
)

const (
	shotWidth  = 118
	shotHeight = 36
)

func main() {
	scene := "recording"
	if len(os.Args) > 1 {
		scene = os.Args[1]
	}
	components.ApplyTheme(components.DefaultTheme(true))
	if _, err := os.Stdout.WriteString(frame(scene)); err != nil {
		os.Exit(1)
	}
}

func frame(scene string) string {
	var (
		tab     components.TabID
		body    string
		footer  string
		rec     bool
		elapsed time.Duration
	)
	switch scene {
	case "doctor":
		tab = components.TabDoctor
		body = doctorScene().View()
		footer = components.FooterForTab(tab, false, 0, components.DoctorFooterNormal, 0, "", "", shotWidth)
	case "config":
		tab = components.TabConfig
		body = configScene().View()
		footer = components.FooterForTab(tab, false, 0, 0, components.ConfigFooterNormal, "", "", shotWidth)
	case "transcribe":
		tab = components.TabHome
		body = transcribeScene().View()
		footer = components.FooterForTab(tab, false, components.SessionsFooterTranscribing, 0, 0, "", "", shotWidth)
	default:
		tab = components.TabHome
		rec = true
		elapsed = 12*time.Minute + 47*time.Second
		body = recordingScene().View()
		footer = components.FooterForTab(tab, false, components.SessionsFooterNormal, 0, 0, "", "", shotWidth)
	}

	var b string
	b += components.Header("Linux · PipeWire", rec, elapsed, false) + "\n"
	b += components.TabBar(tab) + "\n\n"
	b += body + "\n"
	b += footer
	return components.PadView(b, shotWidth, shotHeight)
}

func recordingScene() components.HomeView {
	sessions := demoSessions()
	sessions.Cursor = 0
	sessions.PreviewText = demoTranscript
	return components.HomeView{
		AppState:       "recording",
		Provider:       "Google Meet",
		SystemDevice:   "alsa_output.pci-0000_00_1f.3.analog-stereo.monitor",
		MicDevice:      "Built-in Audio Analog Stereo",
		Recording:      true,
		Duration:       12*time.Minute + 47*time.Second,
		SessionDir:     "~/Music/anoted/2026-09-01_14-02-11_google_meet",
		AutoRecord:     false,
		Width:          shotWidth,
		Height:         shotHeight,
		SystemBands:    demoBands(0.72),
		MicBands:       demoBands(0.48),
		LevelFrame:     12,
		LevelEnabled:   true,
		LevelAvailable: true,
		Sessions:       sessions,
	}
}

func transcribeScene() components.HomeView {
	sessions := demoSessions()
	sessions.Cursor = 1
	sessions.TranscribeActive = true
	sessions.TranscribeSessionDir = sessions.PageRecords[1].Dir
	sessions.TranscribePercent = 64
	sessions.TranscribeETA = 18 * time.Second
	sessions.TranscribeLog = []string{
		"backend: faster-whisper  model: turbo  device: cuda",
		"[00:00:04] Okay, let's start with the hiring plan.",
		"[00:00:11] We still need two backend engineers before October.",
		"[00:00:19] I'll send the scorecard after this call.",
	}
	return components.HomeView{
		AppState:       "idle",
		Provider:       "None detected",
		SystemDevice:   "alsa_output.pci-0000_00_1f.3.analog-stereo.monitor",
		MicDevice:      "Built-in Audio Analog Stereo",
		Width:          shotWidth,
		Height:         shotHeight,
		SystemBands:    demoBands(0.08),
		MicBands:       demoBands(0.05),
		LevelEnabled:   true,
		LevelAvailable: true,
		Sessions:       sessions,
	}
}

func doctorScene() components.DoctorView {
	return components.DoctorView{
		AppState:     "idle",
		Platform:     "Linux",
		Backend:      "pipewire",
		Provider:     "None detected",
		SystemDevice: "alsa_output.pci-0000_00_1f.3.analog-stereo.monitor",
		MicDevice:    "Built-in Audio Analog Stereo",
		Width:        shotWidth,
		Height:       shotHeight,
		Report: doctor.Report{
			Platform: platform.Info{OS: platform.OSLinux, DisplayName: "Linux", Session: "wayland"},
			Checks: []doctor.Check{
				{Name: "anoted_version", Status: "ok", Detail: "v0.1.0"},
				{Name: "operating_system", Status: "ok", Detail: "Linux"},
				{Name: "wsl2", Status: "ok", Detail: "not WSL2"},
				{Name: "output_dir", Status: "ok", Detail: "~/Music/anoted"},
				{Name: "pw-cat", Status: "ok", Detail: "/usr/bin/pw-cat"},
				{Name: "ffmpeg", Status: "ok", Detail: "/usr/bin/ffmpeg"},
				{Name: "recorder_backend", Status: "ok", Detail: "pipewire"},
				{Name: "whisper", Status: "ok", Detail: "faster-whisper (managed venv)"},
				{Name: "tray_indicator", Status: "ok", Detail: "snixembed running"},
			},
		},
	}
}

func configScene() components.ConfigMenuView {
	boolRow := func(label, value string, selected bool) components.ConfigFieldRow {
		return components.ConfigFieldRow{Label: label, Value: value, Selected: selected, Kind: "bool"}
	}
	textRow := func(label, value string) components.ConfigFieldRow {
		return components.ConfigFieldRow{Label: label, Value: value, Kind: "text"}
	}
	return components.ConfigMenuView{
		Path:   "~/.config/anoted/config.yaml",
		Width:  shotWidth,
		Height: shotHeight,
		Sections: []components.ConfigSectionPanel{
			{
				Label:   "General",
				Focused: true,
				Fields: []components.ConfigFieldRow{
					textRow("output_dir", "~/Music/anoted"),
					boolRow("auto_record", "false", true),
					boolRow("launch_at_login", "false", false),
					boolRow("tray_indicator", "true", false),
					boolRow("auto_record_requires_confirmation", "true", false),
				},
			},
			{Label: "Audio"},
			{Label: "Detection"},
			{Label: "Transcription"},
			{Label: "Desktop"},
			{Label: "Privacy"},
		},
	}
}

func demoSessions() components.SessionsView {
	t1 := time.Date(2026, 9, 1, 14, 2, 11, 0, time.Local)
	t2 := time.Date(2026, 8, 28, 9, 15, 0, 0, time.Local)
	t3 := time.Date(2026, 8, 21, 16, 40, 0, 0, time.Local)
	dir1 := "/home/you/Music/anoted/2026-09-01_14-02-11_google_meet"
	dir2 := "/home/you/Music/anoted/2026-08-28_09-15-00_teams"
	dir3 := "/home/you/Music/anoted/2026-08-21_16-40-00_google_meet"
	return components.SessionsView{
		Page:           1,
		PageCount:      1,
		TotalCount:     3,
		Width:          shotWidth,
		Height:         shotHeight,
		CurrentOpener:  "auto",
		OpenerDetected: "dolphin",
		PageRecords: []session.Record{
			{
				ID: 12, Dir: dir1, Provider: session.ProviderGoogleMeet,
				Platform: "linux", Backend: "pipewire", StartedAt: t1,
				Status:   session.StatusActive,
				Metadata: session.Metadata{Duration: "12m47s", Provider: session.ProviderGoogleMeet},
			},
			{
				ID: 11, Dir: dir2, Provider: session.ProviderTeams,
				Platform: "linux", Backend: "pipewire", StartedAt: t2, EndedAt: t2.Add(48 * time.Minute),
				Status:   session.StatusStopped,
				Metadata: session.Metadata{Duration: "48m12s", Provider: session.ProviderTeams},
			},
			{
				ID: 10, Dir: dir3, Provider: session.ProviderGoogleMeet,
				Platform: "linux", Backend: "pipewire", StartedAt: t3, EndedAt: t3.Add(31 * time.Minute),
				Status:   session.StatusStopped,
				Metadata: session.Metadata{Duration: "31m05s", Provider: session.ProviderGoogleMeet},
			},
		},
		Artifacts: map[string]components.SessionArtifacts{
			dir1: {HasAudio: true},
			dir2: {HasAudio: true, HasTranscript: true},
			dir3: {HasAudio: true, HasTranscript: true},
		},
		PreviewText: demoTranscript,
	}
}

func demoBands(peak float64) []float64 {
	out := make([]float64, 28)
	for i := range out {
		// A gentle mid-heavy curve so the equalizer looks alive, not random noise.
		x := float64(i) / float64(len(out)-1)
		hump := 1 - (x-0.45)*(x-0.45)*3.2
		if hump < 0.12 {
			hump = 0.12
		}
		ripple := 0.18 * (0.5 + 0.5*float64((i*3)%5)/4)
		v := peak * (0.55*hump + ripple)
		if v > 1 {
			v = 1
		}
		out[i] = v
	}
	return out
}

const demoTranscript = `We aligned on shipping the beta by Friday.
Alex will own the Windows installer notes.
Sam follows up with legal on the recording consent blurb.
Next check-in: Thursday 10:00.`
