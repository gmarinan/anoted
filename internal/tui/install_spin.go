package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

// installSpinInterval is fast enough to read as motion without being a
// meaningful wakeup cost — it only runs while an install is in flight.
const installSpinInterval = 120 * time.Millisecond

type installSpinMsg struct{}

// installBusy reports whether any long-running install is on screen.
func (m Model) installBusy() bool {
	return m.whisperInstallActive || m.gpuInstallActive || m.setupWizard.Busy
}

// scheduleInstallSpin keeps the spinner turning while something is installing.
//
// pip can go minutes between log lines. With a static label and no repaint the
// app looked hung exactly when a user is most tempted to kill it — and killing
// it mid-install is how you end up with a half-built venv.
func (m Model) scheduleInstallSpin() tea.Cmd {
	if !m.installBusy() {
		return nil
	}
	return tea.Tick(installSpinInterval, func(time.Time) tea.Msg { return installSpinMsg{} })
}

func (m Model) handleInstallSpin() (tea.Model, tea.Cmd) {
	if !m.installBusy() {
		return m, nil
	}
	m.installFrame++
	return m, m.scheduleInstallSpin()
}
