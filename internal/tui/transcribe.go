package tui

import (
	"context"
	"errors"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"meetctl/internal/transcribe"
)

const transcribeLogMax = 30
const transcribeBlinkInterval = 500 * time.Millisecond

type transcribeProgressMsg struct {
	percent     float64
	segmentText string
	eta         time.Duration
}

type transcribeResultMsg struct {
	result     transcribe.Result
	err        error
	sessionDir string
}

type transcribeEnvelopeMsg struct {
	ch    <-chan tea.Msg
	inner tea.Msg
}

type transcribeBlinkMsg struct{}

func transcribeSessionCmd(m Model, sessionDir string, ctx context.Context) tea.Cmd {
	svc := m.deps.Transcriber
	ch := make(chan tea.Msg, 8)
	started := time.Now()

	go func() {
		res, err := svc.TranscribeSessionWithProgress(ctx, sessionDir, func(p transcribe.Progress) {
			if ctx.Err() != nil {
				return
			}
			eta := transcribe.ComputeETA(p.Percent, time.Since(started))
			select {
			case ch <- transcribeProgressMsg{percent: p.Percent, segmentText: p.SegmentText, eta: eta}:
			default:
			case <-ctx.Done():
			}
		})
		sendTranscribeMsg(ch, ctx, transcribeResultMsg{result: res, err: err, sessionDir: sessionDir})
	}()

	return waitTranscribeMsg(ch)
}

func sendTranscribeMsg(ch chan<- tea.Msg, ctx context.Context, msg tea.Msg) {
	select {
	case ch <- msg:
	case <-ctx.Done():
	}
	close(ch)
}

func waitTranscribeMsg(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return transcribeResultMsg{err: context.Canceled}
		}
		return transcribeEnvelopeMsg{ch: ch, inner: msg}
	}
}

func (m Model) scheduleTranscribeBlink() tea.Cmd {
	if !m.transcribeActive {
		return nil
	}
	return tea.Tick(transcribeBlinkInterval, func(time.Time) tea.Msg { return transcribeBlinkMsg{} })
}

func (m Model) appendTranscribeLog(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	const max = 120
	if len(line) > max {
		line = line[:max-1] + "…"
	}
	m.transcribeLog = append(m.transcribeLog, line)
	if len(m.transcribeLog) > transcribeLogMax {
		m.transcribeLog = m.transcribeLog[len(m.transcribeLog)-transcribeLogMax:]
	}
}

func (m Model) clearTranscribeCancel() {
	if m.transcribeCancel != nil {
		m.transcribeCancel = nil
	}
}

func (m Model) cancelTranscribeJob() {
	if m.transcribeCancel != nil {
		m.transcribeCancel()
		m.transcribeCancel = nil
	}
}

func (m Model) handleTranscribeEnvelope(msg transcribeEnvelopeMsg) (tea.Model, tea.Cmd) {
	switch inner := msg.inner.(type) {
	case transcribeProgressMsg:
		if inner.percent > m.transcribePercent {
			m.transcribePercent = inner.percent
		}
		if inner.eta > 0 {
			m.transcribeETA = inner.eta
		}
		if inner.segmentText != "" {
			m.appendTranscribeLog(inner.segmentText)
		}
		return m, waitTranscribeMsg(msg.ch)
	case transcribeResultMsg:
		return m.handleTranscribeResult(inner)
	default:
		return m, waitTranscribeMsg(msg.ch)
	}
}

func (m Model) handleTranscribeResult(msg transcribeResultMsg) (tea.Model, tea.Cmd) {
	m.transcribeActive = false
	m.transcribeBlink = false
	m.clearTranscribeCancel()

	if msg.err != nil {
		if errors.Is(msg.err, context.Canceled) {
			m.transcribeErr = ""
			m.sessionsErr = ""
			m.appendTranscribeLog("transcription stopped")
			return m, nil
		}
		m.transcribeErr = msg.err.Error()
		m.transcribeSessionDir = msg.sessionDir
		m.sessionsErr = msg.err.Error()
		return m, nil
	}
	m.transcribeErr = ""
	m.sessionsErr = ""
	m.transcribePercent = 100
	m.transcribeSessionDir = ""
	if m.screen == ScreenSessions {
		recs, err := loadSessionRecords(m.deps.Store)
		if err == nil {
			m.sessions = recs
		}
	}
	return m, nil
}

func (m Model) startTranscribe(sessionDir string) (Model, tea.Cmd) {
	if m.transcribeActive || sessionDir == "" {
		return m, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.transcribeCancel = cancel
	m.transcribeActive = true
	m.transcribeSessionDir = sessionDir
	m.transcribePercent = 0
	m.transcribeETA = 0
	m.transcribeLog = nil
	m.transcribeErr = ""
	m.transcribeBlink = true
	m.sessionsErr = ""
	return m, tea.Batch(transcribeSessionCmd(m, sessionDir, ctx), m.scheduleTranscribeBlink())
}

func (m Model) stopTranscribe() (Model, tea.Cmd) {
	if !m.transcribeActive {
		return m, nil
	}
	m.appendTranscribeLog("stopping…")
	m.cancelTranscribeJob()
	return m, nil
}

func (m Model) handleTranscribeBlink() (tea.Model, tea.Cmd) {
	if !m.transcribeActive {
		return m, nil
	}
	m.transcribeBlink = !m.transcribeBlink
	return m, m.scheduleTranscribeBlink()
}

// cancelTranscribeOnQuit kills any in-flight whisper subprocess before exit.
func (m Model) cancelTranscribeOnQuit() Model {
	if m.transcribeActive {
		m.cancelTranscribeJob()
		m.transcribeActive = false
		m.transcribeBlink = false
	}
	return m
}
