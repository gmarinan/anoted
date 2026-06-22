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
	dimBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("238")).
			Padding(0, 1)
	boxTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	modalBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("63")).
			Background(lipgloss.Color("235")).
			Padding(0, 2)
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
	// TX status colors
	txDoneStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("42"))
	txPendingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	txActiveStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
	txActiveAltStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("220"))
	txErrorStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("203"))
	footerBarStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(lipgloss.Color("238")).
			Padding(0, 0)
)

// Nerd Font icons (requires a Nerd Font in the terminal).
const (
	IconRecording   = "\U000F0376C" // 󰍬
	IconAudioSaved  = "\U000F09BA8" // 󰦨
	IconTranscribed = "\U000F0BB78" // 󰮸
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
	return renderBox(title, content, width, boxStyle, boxTitleStyle)
}

// DimBox renders a titled panel with a dim border for unfocused sections.
func DimBox(title, content string, width int) string {
	return renderBox(title, content, width, dimBoxStyle, boxTitleStyle)
}

func renderBox(title, content string, width int, style, titleStyle lipgloss.Style) string {
	if width < 12 {
		width = 40
	}
	titleLine := titleStyle.Render(strings.ToUpper(title))
	body := content
	if body == "" {
		body = subtleStyle.Render("(empty)")
	}
	inner := titleLine + "\n" + body
	return style.Width(width).Render(inner)
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

// FooterBar renders a single-line footer bar (htop/lazygit style).
func FooterBar(hints string, width int) string {
	if width > 0 {
		return footerBarStyle.Width(width).Render(hints)
	}
	return footerBarStyle.Render(hints)
}

// FooterWithTrailingStatus places status text flush-right on the footer line.
func FooterWithTrailingStatus(hints, status string, width int) string {
	if status == "" {
		return hints
	}
	if width <= 0 {
		return JoinFooter(hints, status)
	}
	gap := width - lipgloss.Width(hints) - lipgloss.Width(status)
	if gap < 2 {
		return JoinFooter(hints, status)
	}
	return hints + strings.Repeat(" ", gap) + status
}

// FloatCenter overlays content centered on a dimmed background.
func FloatCenter(background, overlay string, width, height int) string {
	if strings.TrimSpace(overlay) == "" {
		return background
	}

	bgLines := strings.Split(strings.TrimRight(background, "\n"), "\n")
	olLines := strings.Split(strings.TrimRight(overlay, "\n"), "\n")

	modalH := len(olLines)
	modalW := 0
	for _, line := range olLines {
		if w := lipgloss.Width(line); w > modalW {
			modalW = w
		}
	}

	bgW, bgH := 0, len(bgLines)
	for _, line := range bgLines {
		if w := lipgloss.Width(line); w > bgW {
			bgW = w
		}
	}

	if width <= 0 {
		width = modalW
	}
	if bgW > width {
		width = bgW
	}
	if height <= 0 {
		height = bgH
	}
	if bgH > height {
		height = bgH
	}

	startY := (height - modalH) / 2
	if startY < 0 {
		startY = 0
	}
	startX := (width - modalW) / 2
	if startX < 0 {
		startX = 0
	}
	if startY+modalH > height {
		height = startY + modalH
	}
	if startX+modalW > width {
		width = startX + modalW
	}

	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	dimmedLines := make([]string, height)
	for y := 0; y < height; y++ {
		if y < len(bgLines) {
			dimmedLines[y] = dimStyle.Render(bgLines[y])
		} else {
			dimmedLines[y] = ""
		}
	}

	bgLayer := lipgloss.NewLayer(strings.Join(dimmedLines, "\n"))
	olLayer := lipgloss.NewLayer(overlay).X(startX).Y(startY)
	return lipgloss.NewCompositor(bgLayer, olLayer).Render()
}

// PickerModal renders a compact floating picker dialog.
func PickerModal(title string, body string, maxWidth int) string {
	lines := strings.Split(body, "\n")
	modalW := lipgloss.Width(boxTitleStyle.Render(strings.ToUpper(title))) + 4
	for _, line := range lines {
		if w := lipgloss.Width(line); w+4 > modalW {
			modalW = w + 4
		}
	}
	if modalW < 28 {
		modalW = 28
	}
	if maxWidth > 0 && modalW > maxWidth {
		modalW = maxWidth
	}
	inner := boxTitleStyle.Render(strings.ToUpper(title)) + "\n" + body
	return modalBoxStyle.Width(modalW).Render(inner)
}
