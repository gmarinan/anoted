package tui

import (
	"log/slog"
	"time"

	tea "charm.land/bubbletea/v2"
)

const autoRecordRetryDelay = 2 * time.Second
const maxAutoRecordFailures = 3
const autoRecordGiveUpMsg = "Auto-record failed — press r to retry"

// autoRecordStartAction describes what handleDetection should do for auto-record.
type autoRecordStartAction int

const (
	autoRecordNoop autoRecordStartAction = iota
	autoRecordWait
	autoRecordConfirm
	autoRecordStart
)

// mayResumeWithoutConfirm reports whether the pending resume was granted by the
// meeting we are currently looking at. Scoping it this way is what keeps a
// stale flag from silently bypassing auto_record_requires_confirmation for a
// different meeting later on.
func (m Model) mayResumeWithoutConfirm() bool {
	if !m.wantAutoRecordResume || m.resumeForSessionKey == "" {
		return false
	}
	return m.resumeForSessionKey == meetingSessionKey(m.detection)
}

func (m Model) autoRecordAction(now time.Time, newSession bool) autoRecordStartAction {
	force := m.mayResumeWithoutConfirm()
	var action autoRecordStartAction

	switch {
	case !m.autoRecord || m.recording:
		action = autoRecordNoop
	case m.recordOpInFlight:
		action = autoRecordWait
	case !m.autoRecordRetryAfter.IsZero() && now.Before(m.autoRecordRetryAfter):
		action = autoRecordWait
	case m.autoRecordFailures >= maxAutoRecordFailures:
		action = autoRecordNoop
	case m.deps.Config.AutoRecordRequiresConfirmation && !force && m.recordConfirmDismissed:
		action = autoRecordNoop
	case m.deps.Config.AutoRecordRequiresConfirmation && !force:
		action = autoRecordConfirm
	case !force && !newSession && shouldBlockAutoStart(false, m.lastAutoStopAt, now):
		action = autoRecordWait
	default:
		action = autoRecordStart
	}

	if action != autoRecordNoop || m.autoRecord {
		logAutoRecordDecision(m, action, newSession, force)
	}
	return action
}

func logAutoRecordDecision(m Model, action autoRecordStartAction, newSession, force bool) {
	slog.Debug("auto-record decision",
		"action", autoRecordActionName(action),
		"auto_record", m.autoRecord,
		"recording", m.recording,
		"in_meeting", m.detection.InMeeting,
		"provider", m.detection.Provider,
		"title", m.detection.Title,
		"want_resume", m.wantAutoRecordResume,
		"new_session", newSession,
		"force", force,
		"op_in_flight", m.recordOpInFlight,
		"confirm_dismissed", m.recordConfirmDismissed,
		"failures", m.autoRecordFailures,
	)
}

func autoRecordActionName(a autoRecordStartAction) string {
	switch a {
	case autoRecordNoop:
		return "noop"
	case autoRecordWait:
		return "wait"
	case autoRecordConfirm:
		return "confirm"
	case autoRecordStart:
		return "start"
	default:
		return "unknown"
	}
}

func (m Model) dispatchAutoRecordStart(now time.Time) (Model, tea.Cmd) {
	m.recordOpInFlight = true
	m.recordOpAt = now
	m.statusNote = ""
	m.errMsg = ""
	return m, startRecordingCmd(m)
}

func (m Model) scheduleAutoRecordRetry() tea.Cmd {
	return tea.Tick(autoRecordRetryDelay, func(time.Time) tea.Msg {
		return autoRecordRetryMsg{}
	})
}

type autoRecordRetryMsg struct{}
