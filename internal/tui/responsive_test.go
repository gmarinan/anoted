package tui

// Every screen, in its common states, at terminal sizes from 60x20 to 240x70.
// Home used to budget its table with a fixed 25 rows of chrome when the real
// figure was 31 to 47, so on 80x24, 100x30 and 120x40 the frame ran past the
// bottom of the terminal; the help overlay was 52 rows at every size; and
// Config's section tabs were 116 cells wide in its narrow layout.

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"anoted/internal/config"
	"anoted/internal/doctor"
	"anoted/internal/filejob"
	"anoted/internal/platform"
	"anoted/internal/recorder"
	"anoted/internal/session"
	"anoted/internal/transcribe"
	"github.com/charmbracelet/x/ansi"
)

var responsiveSizes = [][2]int{{60, 20}, {80, 24}, {100, 30}, {120, 40}, {160, 50}, {240, 70}}

func responsiveSessions() []session.Record {
	base := time.Date(2026, 11, 20, 10, 0, 0, 0, time.Local)
	provs := []session.Provider{session.ProviderGoogleMeet, session.ProviderTeams, session.ProviderUnknown}
	var out []session.Record
	for i := 0; i < 23; i++ {
		st := base.Add(-time.Duration(i) * 26 * time.Hour)
		p := provs[i%3]
		out = append(out, session.Record{
			ID: int64(100 - i), Dir: fmt.Sprintf("/home/you/Music/anoted/2026-11-%02d_10-00-00_%s", 20-i%20, p),
			Provider: p, Platform: "linux", Backend: "pipewire", StartedAt: st, EndedAt: st.Add(47 * time.Minute),
			Status:   session.StatusStopped,
			Metadata: session.Metadata{Duration: "47m12s", Provider: p},
		})
	}
	return out
}

func responsiveBase(w, h int) Model {
	cfg := config.Default()
	cfg.Transcription.Backend = transcribe.BackendParakeet
	m := Model{
		screen: ScreenMain, width: w, height: h,
		scroll: newSessionScroll(), cache: &viewCache{},
		deps: Deps{Config: cfg, ConfigPath: "/home/you/.config/anoted/config.yaml", FilesRoot: "/home/you/.local/share/anoted/files",
			Downloads: &fakeDownloads{}, Recorder: recorder.NewDummyRecorder(),
			Platform: platform.Info{OS: platform.OSLinux, DisplayName: "Linux", Session: "x11"}},
		sessions:       responsiveSessions(),
		systemDevice:   "alsa_output.pci-0000_00_1f.3.analog-stereo.monitor",
		micDevice:      "Built-in Audio Analog Stereo",
		appState:       "idle",
		levelAvailable: true,
		filesRootLabel: "~/.local/share/anoted/files",
		previewText:    strings.Repeat("Okay, let's start with the hiring plan. We still need two backend engineers. ", 8),
	}
	return m
}

type responsiveScene struct {
	name string
	mk   func(w, h int) Model
}

func responsiveScenes() []responsiveScene {
	scenes := []responsiveScene{
		{"home", func(w, h int) Model { return responsiveBase(w, h) }},
		{"home-empty", func(w, h int) Model { m := responsiveBase(w, h); m.sessions = nil; m.previewText = ""; return m }},
		{"home-recording", func(w, h int) Model {
			m := responsiveBase(w, h)
			m.recording = true
			m.appState = "recording"
			m.recordElapsed = 12*time.Minute + 47*time.Second
			m.sessionDir = "/home/you/Music/anoted/2026-11-22_14-02-11_google_meet"
			m.provider = "google_meet"
			m.systemBands = make([]float64, 24)
			m.micBands = make([]float64, 24)
			for i := range m.systemBands {
				m.systemBands[i] = float64(i%7) / 7
				m.micBands[i] = float64(i%5) / 5
			}
			return m
		}},
		{"home-transcribing", func(w, h int) Model {
			m := responsiveBase(w, h)
			m.transcribeActive = true
			m.transcribeSessionDir = m.sessions[0].Dir
			m.transcribePercent = 64
			m.transcribeETA = 18 * time.Second
			m.transcribeLog = []string{"backend: parakeet  model: parakeet-tdt-0.6b-v3  device: cuda", "[00:00:04] Okay, let's start.", "[00:00:11] We still need two backend engineers before October, and a designer."}
			m.transcribeQueue = []string{m.sessions[1].Dir, m.sessions[2].Dir}
			return m
		}},
		{"home-help", func(w, h int) Model { m := responsiveBase(w, h); m.helpOpen = true; return m }},
		{"home-quit", func(w, h int) Model { m := responsiveBase(w, h); m.quitConfirmOpen = true; return m }},
		{"doctor", func(w, h int) Model {
			m := responsiveBase(w, h)
			m.screen = ScreenDoctor
			names := []string{"anoted_version", "operating_system", "output_dir", "pw-cat", "ffmpeg", "recorder_backend", "system_monitor", "microphone", "detector", "whisper", "transcription_model", "gpu", "torch_cuda", "autostart"}
			for i, n := range names {
				st := "ok"
				if i%5 == 4 {
					st = "warn"
				}
				m.doctorReport.Checks = append(m.doctorReport.Checks, doctor.Check{Name: n, Status: st, Detail: "/usr/bin/something with a fairly long explanation of what was found here"})
			}
			return m
		}},
		{"files-empty", func(w, h int) Model { m := responsiveBase(w, h); m.screen = ScreenFiles; return m }},
		{"files", func(w, h int) Model {
			m := responsiveBase(w, h)
			m.screen = ScreenFiles
			for i := 0; i < 9; i++ {
				m.fileJobs = append(m.fileJobs, filejob.Job{Dir: fmt.Sprintf("/x/%d", i), Name: fmt.Sprintf("WhatsApp Audio 2026-11-%02d at 10.15.32 entrevista larga.opus", i+1), Done: i > 1, DurationSec: 1234, AddedAt: time.Now()})
			}
			m.fileJobs[1].Error = "parakeet: CUDA out of memory while decoding chunk 3"
			m.fileCursor = 2
			m = m.setFileText("/x/2", strings.Repeat("Hola, ¿qué tal? Esto es una transcripción de prueba bastante larga. ", 40))
			return m
		}},
	}
	for s := 0; s < configSectionCount; s++ {
		scenes = append(scenes, responsiveScene{fmt.Sprintf("config-%d-%s", s, configSectionLabels[s]), func(w, h int) Model {
			m := responsiveBase(w, h)
			m.screen = ScreenConfig
			m.configSection = s
			if s == modelsSection {
				m.downloadsScanned = true
				for _, d := range []transcribe.Download{
					{ID: "whisper-runtime", Label: "Whisper runtime", Path: "/home/you/.local/share/anoted/whisper-venv", Bytes: 7 << 30},
					{ID: "faster-whisper-large-v3", Label: "faster-whisper large-v3", Path: "/home/you/.cache/huggingface/hub/models--Systran--faster-whisper-large-v3", Bytes: 3 << 30, Shared: true},
					{ID: "parakeet-model", Label: "Parakeet TDT 0.6B v3 model", Path: "/home/you/.local/share/anoted/models/parakeet-tdt-0.6b-v3-onnx", Bytes: 2 << 30, InUse: true},
				} {
					m.downloads = append(m.downloads, downloadEntry{Download: d, display: filesRootDisplay(d.Path)})
				}
			}
			return m
		}})
	}
	return scenes
}

func TestEveryScreenFitsTheTerminal(t *testing.T) {
	for _, sc := range responsiveScenes() {
		for _, sz := range responsiveSizes {
			w, h := sz[0], sz[1]
			frame := ansi.Strip(sc.mk(w, h).View().Content)
			lines := strings.Split(frame, "\n")
			if len(lines) != h {
				t.Errorf("%s at %dx%d: %d rows", sc.name, w, h, len(lines))
			}
			for i, l := range lines {
				if lw := ansi.StringWidth(l); lw > w {
					t.Errorf("%s at %dx%d: row %d is %d cells", sc.name, w, h, i+1, lw)
					break
				}
			}
			// 60x20 is below what the layouts are built for; everything from
			// the classic 80x24 up must fit without the overflow notice.
			if w >= 80 && strings.Contains(frame, "more rows") {
				t.Errorf("%s at %dx%d overflows:\n%s", sc.name, w, h, frame)
			}
		}
	}
}

func TestHomeKeepsTheSelectionWhenThePageSizeChanges(t *testing.T) {
	m := responsiveBase(120, 40)
	m = m.sessionsNavigate(9) // #91
	want, _ := m.selectedSession()
	m.pageSizeSeen = m.sessionsPageSize()

	// Recording adds rows to the status panel, which shrinks the page.
	m.recording = true
	m.sessionDir = "/home/you/Music/anoted/now"
	before := m.pageSizeSeen
	m = m.repage()
	if m.pageSizeSeen == before {
		t.Fatalf("page size did not change (%d)", before)
	}
	if got, _ := m.selectedSession(); got.ID != want.ID {
		t.Fatalf("selection moved from #%d to #%d", want.ID, got.ID)
	}
}
