package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"anoted/internal/transcribe"
	"anoted/internal/tui/components"
	tea "charm.land/bubbletea/v2"
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
			// Progress is best-effort: drop the update rather than block the
			// transcriber if the UI is behind. The ctx.Done case that used to
			// sit here was unreachable — a select with a default never blocks,
			// so it only made the cancellation path look handled. The real
			// check is the ctx.Err() guard above.
			select {
			case ch <- transcribeProgressMsg{percent: p.Percent, segmentText: p.SegmentText, eta: eta}:
			default:
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

// Pointer receiver: with a value receiver every appended line was written to a
// discarded copy of the Model, so the transcription preview showed only its
// seed line for the whole run.
func (m *Model) appendTranscribeLog(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	line = components.ClampLogLine(line)
	m.transcribeLog = append(m.transcribeLog, line)
	if len(m.transcribeLog) > transcribeLogMax {
		m.transcribeLog = m.transcribeLog[len(m.transcribeLog)-transcribeLogMax:]
	}
}

// clearTranscribeCancel releases the context after the job has finished on its
// own. Dropping the reference without calling it leaks the context's resources.
func (m *Model) clearTranscribeCancel() {
	if m.transcribeCancel != nil {
		m.transcribeCancel()
		m.transcribeCancel = nil
	}
}

func (m *Model) cancelTranscribeJob() {
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
	// A finished run creates transcript files, so the cached artifacts and the
	// preview are both stale.
	m.sessionArtifacts = gatherSessionFacts(m.sessions, m.deps.Config.Transcription)
	m.previewDir = ""
	m = m.refreshPreview()

	if msg.err != nil && errors.Is(msg.err, context.Canceled) {
		// Stop means stop: a cancelled job must not drain the rest of the queue.
		m.transcribeQueue = nil
		m.transcribeErr = ""
		m.transcribeErrDir = ""
		m.sessionsErr = ""
		m.transcribeSessionDir = ""
		m.appendTranscribeLog("transcription stopped")
		return m, nil
	}

	if msg.err != nil {
		m.transcribeErr = msg.err.Error()
		m.transcribeErrDir = msg.sessionDir
		// sessionsErr replaces the whole sessions table with the message. That
		// is right when nothing else is waiting; with a queue behind this one
		// it would hide the meetings that still have to run.
		if len(m.transcribeQueue) == 0 {
			m.transcribeSessionDir = msg.sessionDir
			m.sessionsErr = msg.err.Error()
			return m, nil
		}
		m.sessionsErr = ""
	} else {
		if m.transcribeErrDir == msg.sessionDir {
			m.transcribeErr = ""
			m.transcribeErrDir = ""
		}
		m.sessionsErr = ""
		m.transcribePercent = 100
		m.transcribeSessionDir = ""
	}

	m, nextCmd, started := m.startNextTranscribe()
	if started {
		if msg.err != nil {
			// startNextTranscribe clears the error fields; the failed row should
			// stay marked while the rest of the queue runs.
			m.transcribeErr = msg.err.Error()
			m.transcribeErrDir = msg.sessionDir
			m.appendTranscribeLog("· previous session failed — continuing the queue")
		}
		var cmds []tea.Cmd
		cmds = append(cmds, nextCmd)
		if msg.err == nil && m.screen == ScreenMain {
			cmds = append(cmds, m.loadSessionsCmd())
		}
		return m, tea.Batch(cmds...)
	}
	if msg.err == nil && m.screen == ScreenMain {
		return m, m.loadSessionsCmd()
	}
	return m, nil
}

// enqueueTranscribe runs sessionDir now, or appends it if a job is already
// active (including one parked until the GPU has free VRAM).
func (m Model) enqueueTranscribe(sessionDir string) (Model, tea.Cmd) {
	sessionDir = strings.TrimSpace(sessionDir)
	if sessionDir == "" {
		return m, nil
	}
	if m.transcribeActive && m.transcribeSessionDir == sessionDir {
		return m, nil
	}
	if transcribeQueued(m.transcribeQueue, sessionDir) {
		return m, nil
	}
	if !m.transcribeActive {
		return m.startTranscribe(sessionDir)
	}
	m.transcribeQueue = append(m.transcribeQueue, sessionDir)
	m.appendTranscribeLog(fmt.Sprintf("· queued — %d waiting", len(m.transcribeQueue)))
	return m, nil
}

func transcribeQueued(queue []string, dir string) bool {
	for _, queued := range queue {
		if queued == dir {
			return true
		}
	}
	return false
}

func (m Model) startNextTranscribe() (Model, tea.Cmd, bool) {
	if m.transcribeActive || len(m.transcribeQueue) == 0 {
		return m, nil, false
	}
	next := m.transcribeQueue[0]
	rest := m.transcribeQueue[1:]
	m.transcribeQueue = rest
	m, cmd := m.startTranscribe(next)
	if cmd == nil {
		m.transcribeQueue = append([]string{next}, rest...)
		return m, nil, false
	}
	return m, cmd, true
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
	m.transcribeErrDir = ""
	m.transcribeBlink = true
	m.sessionsErr = ""
	if n := len(m.transcribeQueue); n > 0 {
		m.appendTranscribeLog(fmt.Sprintf("· %d more queued after this one", n))
	}
	return m, tea.Batch(transcribeSessionCmd(m, sessionDir, ctx), m.scheduleTranscribeBlink())
}

func (m Model) stopTranscribe() (Model, tea.Cmd) {
	if !m.transcribeActive {
		return m, nil
	}
	n := len(m.transcribeQueue)
	m.transcribeQueue = nil
	if n > 0 {
		m.appendTranscribeLog(fmt.Sprintf("stopping… (%d queued cancelled)", n))
	} else {
		m.appendTranscribeLog("stopping…")
	}
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
	m.transcribeQueue = nil
	if m.transcribeActive {
		m.cancelTranscribeJob()
		m.transcribeActive = false
		m.transcribeBlink = false
	}
	return m
}
