package components

import (
	"strings"
)

// FooterForTab returns context-sensitive footer shortcuts.
func FooterForTab(tab TabID, awaitingConfirm bool) string {
	switch tab {
	case TabHome:
		if awaitingConfirm {
			return JoinFooter(
				FooterHint("y", "start recording"),
				FooterHint("n", "dismiss"),
				FooterHint("1-5", "tabs"),
				FooterHint("q", "quit"),
			)
		}
		return JoinFooter(
			FooterHint("r", "record"),
			FooterHint("a", "auto-record"),
			FooterHint("1-5", "tabs"),
			FooterHint("q", "quit"),
		)
	case TabAudio:
		return JoinFooter(
			FooterHint("↑↓", "navigate"),
			FooterHint("Tab", "section"),
			FooterHint("Enter", "select"),
			FooterHint("R", "refresh"),
			FooterHint("1-5", "tabs"),
			FooterHint("q", "quit"),
		)
	case TabDoctor:
		return JoinFooter(
			FooterHint("R", "refresh"),
			FooterHint("1-5", "tabs"),
			FooterHint("q", "quit"),
		)
	case TabSessions:
		return JoinFooter(
			FooterHint("↑↓", "navigate"),
			FooterHint("o", "open folder"),
			FooterHint("p", "play"),
			FooterHint("R", "refresh"),
			FooterHint("1-5", "tabs"),
			FooterHint("q", "quit"),
		)
	case TabConfig:
		return JoinFooter(
			FooterHint("Ctrl+S", "save"),
			FooterHint("R", "reload"),
			FooterHint("1-5", "tabs"),
			FooterHint("q", "quit"),
		)
	default:
		return JoinFooter(FooterHint("q", "quit"))
	}
}

// ScreenToTab maps internal screen IDs to tab IDs.
func ScreenToTab(screen string) TabID {
	switch screen {
	case "audio":
		return TabAudio
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
	case TabAudio:
		return "audio"
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

// NextTab cycles to the next tab.
func NextTab(current TabID) TabID {
	return TabID((int(current) + 1) % len(tabLabels))
}

// PrevTab cycles to the previous tab.
func PrevTab(current TabID) TabID {
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
