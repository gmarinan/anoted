package components

// SessionsFooterMode selects footer shortcuts on the Sessions screen.
type SessionsFooterMode int

const (
	SessionsFooterNormal SessionsFooterMode = iota
	SessionsFooterTranscribing
	SessionsFooterOpenerPicker
	SessionsFooterDeleteConfirm
)

// DoctorFooterMode selects footer shortcuts on the Doctor screen.
type DoctorFooterMode int

const (
	DoctorFooterNormal DoctorFooterMode = iota
	DoctorFooterCanInstall
	DoctorFooterCanInstallGPU
	DoctorFooterCanInstallBoth
	DoctorFooterInstalling
	DoctorFooterInstallingGPU
)

// FooterForTab returns context-sensitive footer shortcuts.
func FooterForTab(tab TabID, awaitingConfirm bool, sessionsMode SessionsFooterMode, doctorMode DoctorFooterMode, configMode ConfigFooterMode, configSaved, configErr string, width int) string {
	switch tab {
	case TabHome:
		if sessionsMode == SessionsFooterDeleteConfirm {
			return FitFooter(width,
				FooterHint("↑↓", "choose"),
				FooterHint("Enter", "apply"),
				FooterHint("Esc", "cancel"),
				FooterHint("q", "quit"),
			)
		}
		if sessionsMode == SessionsFooterOpenerPicker {
			return FitFooter(width,
				FooterHint("↑↓", "choose"),
				FooterHint("Enter", "apply"),
				FooterHint("Esc", "cancel"),
				FooterHint("q", "quit"),
			)
		}
		if awaitingConfirm {
			return FitFooter(width,
				FooterHint("y", "start recording"),
				FooterHint("n", "dismiss"),
				FooterHint("q", "quit"),
			)
		}
		if sessionsMode == SessionsFooterTranscribing {
			return FitFooter(width,
				FooterHint("s", "stop transcribe"),
				FooterHint("↑↓", "navigate"),
				FooterHint("[ ]", "page"),
				FooterHint("o", "open folder"),
				FooterHint("r", "record"),
				FooterHint("R", "refresh"),
				FooterHint("q", "quit…"),
			)
		}
		return FitFooter(width,
			FooterHint("r", "record"),
			FooterHint("S", "setup"),
			FooterHint("a", "auto-record"),
			FooterHint("↑↓", "navigate"),
			FooterHint("t", "transcribe"),
			FooterHint("o", "open folder"),
			FooterHint("f", "folder opener"),
			FooterHint("p", "play"),
			FooterHint("d", "delete"),
			FooterHint("R", "refresh"),
			FooterHint("q", "quit"),
		)
	case TabDoctor:
		if doctorMode == DoctorFooterInstallingGPU {
			return FitFooter(width,
				FooterHint("installing", "GPU…"),
				FooterHint("PgUp/PgDn", "scroll log"),
				FooterHint("R", "refresh"),
				FooterHint("q", "quit…"),
			)
		}
		if doctorMode == DoctorFooterInstalling {
			return FitFooter(width,
				FooterHint("installing", "whisper…"),
				FooterHint("PgUp/PgDn", "scroll log"),
				FooterHint("R", "refresh"),
				FooterHint("q", "quit…"),
			)
		}
		if doctorMode == DoctorFooterCanInstallBoth {
			return FitFooter(width,
				FooterHint("i", "install whisper"),
				FooterHint("g", "install GPU"),
				FooterHint("R", "refresh"),
				FooterHint("q", "quit"),
			)
		}
		if doctorMode == DoctorFooterCanInstallGPU {
			return FitFooter(width,
				FooterHint("g", "install GPU"),
				FooterHint("R", "refresh"),
				FooterHint("q", "quit"),
			)
		}
		if doctorMode == DoctorFooterCanInstall {
			return FitFooter(width,
				FooterHint("i", "install whisper"),
				FooterHint("R", "refresh"),
				FooterHint("q", "quit"),
			)
		}
		return FitFooter(width,
			FooterHint("S", "setup"),
			FooterHint("R", "refresh"),
			FooterHint("q", "quit"),
		)
	case TabConfig:
		return FooterForConfig(configMode, configSaved, configErr, width)
	default:
		return FitFooter(width, FooterHint("q", "quit"))
	}
}

// ScreenToTab maps internal screen IDs to tab IDs.
func ScreenToTab(screen string) TabID {
	switch screen {
	case "doctor":
		return TabDoctor
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
