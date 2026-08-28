package tui

import (
	"os"
	"path/filepath"

	"anoted/internal/autostart"
	"anoted/internal/config"
	"anoted/internal/open"
	"anoted/internal/recorder"
	"anoted/internal/session"
	"anoted/internal/transcribe"
	"anoted/internal/tui/components"
)

// sessionFacts are the filesystem answers the session table needs.
//
// These were probed inside View: HasTranscript stats several candidate paths
// and audioExists stats one more, per visible row, on every frame — up to
// thirty times a second while the level meter runs. On a home directory over
// NFS or a spun-down disk that stalls the whole render loop. They are gathered
// once per session load now, off the Update loop, and View just reads them.
type sessionFacts struct {
	HasTranscript bool
	HasAudio      bool
}

func gatherSessionFacts(recs []session.Record, cfg config.TranscriptionConfig) map[string]components.SessionArtifacts {
	facts := make(map[string]components.SessionArtifacts, len(recs))
	for _, r := range recs {
		if r.Dir == "" {
			continue
		}
		_, audioErr := os.Stat(filepath.Join(r.Dir, recorder.SessionAudioFile))
		facts[r.Dir] = components.SessionArtifacts{
			HasTranscript: transcribe.HasTranscript(r.Dir, cfg),
			HasAudio:      audioErr == nil,
		}
	}
	return facts
}

// refreshPreview recomputes the transcript preview for the selected session.
//
// View used to read the transcript file on every frame. The preview only
// changes when the cursor moves, the list reloads or a transcription finishes,
// so it is cached against the directory it was read from.
func (m Model) refreshPreview() Model {
	rec, ok := m.selectedSession()
	if !ok || rec.Dir == "" {
		m.previewDir, m.previewText = "", ""
		return m
	}
	if m.transcribeActive && rec.Dir == m.transcribeSessionDir {
		// A run in progress owns that area of the screen.
		m.previewDir, m.previewText = "", ""
		return m
	}
	if !m.sessionArtifacts[rec.Dir].HasTranscript {
		m.previewDir, m.previewText = rec.Dir, ""
		return m
	}
	if m.previewDir == rec.Dir && m.previewText != "" {
		return m
	}
	text, err := transcribe.ReadPreview(rec.Dir, m.deps.Config.Transcription, previewLines)
	if err != nil {
		m.previewDir, m.previewText = rec.Dir, ""
		return m
	}
	m.previewDir, m.previewText = rec.Dir, text
	return m
}

const previewLines = 12

// refreshEnvironment resolves the environment facts View needs.
//
// Called at startup and whenever the config that feeds them changes, rather
// than from the render path. open.Detected alone runs exec.LookPath across up
// to six file managers.
func (m Model) refreshEnvironment() Model {
	m.openerCurrent = open.CurrentOpenerID(m.deps.Config.Desktop)
	m.openerDetected = open.Detected(m.deps.Config.Desktop, open.KindFolder)
	m.autostartAvail = autostart.Available()
	m.autostartOn = autostart.Enabled()
	m.levelAvailable = m.deps.LevelMonitor != nil && m.deps.LevelMonitor.Available()
	return m
}

func (m Model) envFacts() envFacts {
	return envFacts{
		AutostartAvailable: m.autostartAvail,
		AutostartEnabled:   m.autostartOn,
	}
}
