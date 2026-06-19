package components

import (
	"strings"

	"charm.land/lipgloss/v2"
)

const AppSubtitle = "PipeWire Edition"

var (
	headerStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	subtleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	labelStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	valueStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	warnStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	errStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	okStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	recStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("203")).Background(lipgloss.Color("52"))
	boxStyle    = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("63")).
			Padding(0, 1)
	boxTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	magentaStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("213"))
	subTabActiveStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("229")).
				Background(lipgloss.Color("63")).
				Padding(0, 1)
	subTabInactiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("244")).
				Padding(0, 1)
	tabActiveStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("229")).
			Background(lipgloss.Color("63")).
			Padding(0, 1)
	tabInactiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("244")).
				Padding(0, 1)
	keyStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
)

// TabID identifies the active main screen (switch with 1–4, not Tab).
type TabID int

const (
	TabHome TabID = iota
	TabDoctor
	TabSessions
	TabConfig
)

var tabLabels = []string{"Home", "Doctor", "Sessions", "Config"}

// Header renders the app title line.
func Header() string {
	return headerStyle.Render("meetctl") + subtleStyle.Render(" · "+AppSubtitle)
}

// TabBar renders the top main navigation tabs (visual only; use 1–4 to switch).
func TabBar(active TabID) string {
	var parts []string
	for i, label := range tabLabels {
		text := "[" + label + "]"
		if TabID(i) == active {
			parts = append(parts, tabActiveStyle.Render(text))
		} else {
			parts = append(parts, tabInactiveStyle.Render(text))
		}
	}
	return strings.Join(parts, " ")
}

// SubTabBar renders highlighted subsection tabs (e.g. Output / Microphone).
func SubTabBar(labels []string, active int) string {
	var parts []string
	for i, label := range labels {
		text := "[" + label + "]"
		if i == active {
			parts = append(parts, subTabActiveStyle.Render(text))
		} else {
			parts = append(parts, subTabInactiveStyle.Render(text))
		}
	}
	return strings.Join(parts, " ")
}

// Box renders a titled bordered panel.
func Box(title, content string, width int) string {
	if width < 12 {
		width = 40
	}
	titleLine := boxTitleStyle.Render(strings.ToUpper(title))
	body := content
	if body == "" {
		body = subtleStyle.Render("(empty)")
	}
	inner := titleLine + "\n" + body
	return boxStyle.Width(width).Render(inner)
}

// Badge renders a small status pill.
func Badge(text string, kind string) string {
	switch kind {
	case "ok", "running", "ready":
		return okStyle.Render(" " + text + " ")
	case "warn", "default":
		return warnStyle.Render(" " + text + " ")
	case "rec":
		return recStyle.Render(" " + text + " ")
	case "meet":
		return magentaStyle.Render(" " + text + " ")
	default:
		return labelStyle.Render(" " + text + " ")
	}
}

// FooterHint renders a keyboard shortcut in the footer.
func FooterHint(key, action string) string {
	return keyStyle.Render(key) + subtleStyle.Render(" "+action)
}

// JoinFooter joins footer hints with separators.
func JoinFooter(hints ...string) string {
	return subtleStyle.Render(strings.Join(hints, "  ·  "))
}
