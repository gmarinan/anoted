package components

import (
	"strings"
)

// SessionsFooterMode selects footer shortcuts on the Sessions screen.
type SessionsFooterMode int

const (
	SessionsFooterNormal SessionsFooterMode = iota
	SessionsFooterOpenerPicker
	SessionsFooterDeleteConfirm
)

// FooterForTab returns context-sensitive footer shortcuts.
func FooterForTab(tab TabID, awaitingConfirm bool, sessionsMode SessionsFooterMode, configMode ConfigFooterMode, configSaved, configErr string, width int) string {
	switch tab {
	case TabHome:
		if awaitingConfirm {
			return JoinFooter(
				FooterHint("y", "start recording"),
				FooterHint("n", "dismiss"),
				FooterHint("1-4", "screens"),
				FooterHint("q", "quit"),
			)
		}
		return JoinFooter(
			FooterHint("r", "record"),
			FooterHint("a", "auto-record"),
			FooterHint("Tab", "audio section"),
			FooterHint("↑↓", "devices"),
			FooterHint("Enter", "select"),
			FooterHint("R", "refresh audio"),
			FooterHint("1-4", "screens"),
			FooterHint("q", "quit"),
		)
	case TabDoctor:
		return JoinFooter(
			FooterHint("R", "refresh"),
			FooterHint("1-4", "screens"),
			FooterHint("q", "quit"),
		)
	case TabSessions:
		if sessionsMode == SessionsFooterDeleteConfirm {
			return JoinFooter(
				FooterHint("↑↓", "choose"),
				FooterHint("Enter", "apply"),
				FooterHint("Esc", "cancel"),
				FooterHint("1-4", "screens"),
				FooterHint("q", "quit"),
			)
		}
		if sessionsMode == SessionsFooterOpenerPicker {
			return JoinFooter(
				FooterHint("↑↓", "choose"),
				FooterHint("Enter", "apply"),
				FooterHint("Esc", "cancel"),
				FooterHint("q", "quit"),
			)
		}
		return JoinFooter(
			FooterHint("↑↓", "navigate"),
			FooterHint("[ ]", "page"),
			FooterHint("t", "transcribe"),
			FooterHint("o", "open folder"),
			FooterHint("f", "folder opener"),
			FooterHint("p", "play"),
			FooterHint("d", "delete"),
			FooterHint("R", "refresh"),
			FooterHint("1-4", "screens"),
			FooterHint("q", "quit"),
		)
	case TabConfig:
		return FooterForConfig(configMode, configSaved, configErr, width)
	default:
		return JoinFooter(FooterHint("q", "quit"))
	}
}

// ScreenToTab maps internal screen IDs to tab IDs.
func ScreenToTab(screen string) TabID {
	switch screen {
	case "doctor":
		return TabDoctor
	case "sessions":
		return TabSessions
	case "config":
		return TabConfig
	default:
		return TabHome
	}
}

// TabToScreen maps tab IDs to internal screen IDs.
func TabToScreen(tab TabID) string {
	switch tab {
	case TabDoctor:
		return "doctor"
	case TabSessions:
		return "sessions"
	case TabConfig:
		return "config"
	default:
		return "main"
	}
}

// NextScreen cycles to the next main screen.
func NextScreen(current TabID) TabID {
	return TabID((int(current) + 1) % len(tabLabels))
}

// PrevScreen cycles to the previous main screen.
func PrevScreen(current TabID) TabID {
	n := int(current) - 1
	if n < 0 {
		n = len(tabLabels) - 1
	}
	return TabID(n)
}

// Legacy helpers kept for tests.
func row(label, value string) string {
	return labelStyle.Render(label+":") + " " + valueStyle.Render(value)
}

// StatusBadge returns a styled status string.
func StatusBadge(status string) string {
	switch status {
	case "ok":
		return okStyle.Render(status)
	case "warn":
		return warnStyle.Render(status)
	case "fail":
		return errStyle.Render(status)
	default:
		return valueStyle.Render(status)
	}
}

// FormatDoctorLine formats a single doctor check for plain-text output.
func FormatDoctorLine(status, name, detail string) string {
	return strings.Join([]string{"[" + StatusBadge(status) + "]", name + ":", detail}, " ")
}
