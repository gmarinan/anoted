package tui

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"anoted/internal/filejob"
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
	// file is set when the job was a standalone file rather than a session.
	file *fileOutcome
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

	// A file job is queued under its folder, like a session, but its audio
	// lives wherever the user keeps it and its outcome is recorded in the job.
	job, isFile := m.fileJob(sessionDir)
	if isFile {
		// The job carries its own language, so it gets its own service: the
		// shared one is bound to the meeting language.
		cfg := m.deps.Config
		cfg.Transcription.Language = job.Language
		svc = transcribe.New(cfg)
	}

	go func() {
		run := func(onProgress transcribe.ProgressFunc) (transcribe.Result, error) {
			if isFile {
				return svc.TranscribeFileWithProgress(ctx, job.Source, job.Dir, onProgress)
			}
			return svc.TranscribeSessionWithProgress(ctx, sessionDir, onProgress)
		}
		res, err := run(func(p transcribe.Progress) {
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
		msg := transcribeResultMsg{result: res, err: err, sessionDir: sessionDir}
		if isFile {
			msg.file = finishFileJob(job, res, err)
		}
		sendTranscribeMsg(ch, ctx, msg)
	}()

	return waitTranscribeMsg(ch)
}

// finishFileJob records how a file job ended and reads back its transcript.
// It runs on the transcription goroutine so Update does no disk I/O for it.
func finishFileJob(job filejob.Job, res transcribe.Result, err error) *fileOutcome {
	out := &fileOutcome{}
	switch {
	case err == nil:
		job.Error = ""
		job.Done = true
		if res.AudioDuration > 0 {
			job.DurationSec = res.AudioDuration.Seconds()
		}
		if text, readErr := filejob.ReadTranscript(job.Dir); readErr == nil {
			out.text = text
		}
	case errors.Is(err, context.Canceled):
		// Stopped by the user: not a failure, and nothing new to record.
	default:
		job.Error = err.Error()
	}
	if saveErr := job.Save(); saveErr != nil {
		slog.Warn("could not save file job", "dir", job.Dir, "err", saveErr)
	}
	out.job = job
	return out
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
	isFile := msg.file != nil
	if isFile {
		m = m.applyFileOutcome(msg.file)
	} else {
		// A finished run creates transcript files, so the cached artifacts and
		// the preview are both stale.
		m.sessionArtifacts = gatherSessionFacts(m.sessions, m.deps.Config.Transcription)
		m.previewDir = ""
		m = m.refreshPreview()
	}

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

	if msg.err != nil && isFile {
		// The failure is already on the job and shown on the Files tab. It must
		// not reach sessionsErr, which replaces the whole sessions table on
		// Home with the message — for a file that is not even listed there.
		m.transcribeSessionDir = ""
		m.sessionsErr = ""
	} else if msg.err != nil {
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
		if msg.err != nil && !isFile {
			// startNextTranscribe clears the error fields; the failed row should
			// stay marked while the rest of the queue runs.
			m.transcribeErr = msg.err.Error()
			m.transcribeErrDir = msg.sessionDir
			m.appendTranscribeLog("· previous session failed — continuing the queue")
		}
		var cmds []tea.Cmd
		cmds = append(cmds, nextCmd)
		if msg.err == nil && !isFile && m.screen == ScreenMain {
			cmds = append(cmds, m.loadSessionsCmd())
		}
		return m, tea.Batch(cmds...)
	}
	if msg.err == nil && !isFile && m.screen == ScreenMain {
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
