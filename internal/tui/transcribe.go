package tui

import (
	"context"

	tea "charm.land/bubbletea/v2"
	"meetctl/internal/transcribe"
)

func transcribeSessionCmd(m Model, sessionDir string) tea.Cmd {
	svc := m.deps.Transcriber
	return func() tea.Msg {
		res, err := svc.TranscribeSession(context.Background(), sessionDir)
		return transcribeResultMsg{result: res, err: err, sessionDir: sessionDir}
	}
}

type transcribeResultMsg struct {
	result     transcribe.Result
	err        error
	sessionDir string
}

func (m Model) handleTranscribeResult(msg transcribeResultMsg) (tea.Model, tea.Cmd) {
	m.transcribing = false
	if msg.err != nil {
		m.sessionsErr = msg.err.Error()
		m.transcribeNote = ""
		return m, nil
	}
	m.sessionsErr = ""
	m.transcribeNote = "transcribed → " + msg.sessionDir
	if m.screen == ScreenSessions {
		recs, err := loadSessionRecords(m.deps.Store)
		if err == nil {
			m.sessions = recs
		}
	}
	return m, nil
}

func (m Model) startTranscribe(sessionDir string) (tea.Model, tea.Cmd) {
	if m.transcribing || sessionDir == "" {
		return m, nil
	}
	m.transcribing = true
	m.transcribeNote = "transcribing…"
	m.sessionsErr = ""
	return m, transcribeSessionCmd(m, sessionDir)
}
