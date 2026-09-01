package components

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// Theme is the semantic palette every component style derives from. One accent
// carries the brand (tabs, selection, keys, box titles); danger stays red on
// purpose — it marks recording, the one state that must be unmissable.
//
// Values are truecolor; bubbletea v2 downsamples them per terminal profile
// (256 → 16 → mono) via colorprofile, so no fallback tables are needed here.
type Theme struct {
	Accent    color.Color // brand: tabs, selection, keys, titles
	AccentFg  color.Color // readable text on Accent
	Border    color.Color
	BorderDim color.Color
	Text      color.Color
	Muted     color.Color // labels
	Faint     color.Color // hints, de-emphasized chrome
	Success   color.Color
	Warning   color.Color
	Danger    color.Color // recording + errors
	DangerBg  color.Color
	Info      color.Color // meeting/detection accents
	Surface   color.Color // modal fill

	// Equalizer color ramps, bottom row (0) to top row (eqRows-1).
	SystemRamp [eqRows]color.Color
	MicRamp    [eqRows]color.Color
}

// DefaultTheme returns the palette for the detected terminal background.
func DefaultTheme(isDark bool) Theme {
	ld := lipgloss.LightDark(isDark)
	c := func(light, dark string) color.Color {
		return ld(lipgloss.Color(light), lipgloss.Color(dark))
	}
	ramp := func(pairs [eqRows][2]string) [eqRows]color.Color {
		var out [eqRows]color.Color
		for i, p := range pairs {
			out[i] = c(p[0], p[1])
		}
		return out
	}
	return Theme{
		//         light      dark
		Accent:    c("#5A4FCF", "#9D8CFF"),
		AccentFg:  c("#FFFFFF", "#16141F"),
		Border:    c("#A79FDB", "#544C8C"),
		BorderDim: c("#DAD6EE", "#38344F"),
		Text:      c("#2A2833", "#DCDAE8"),
		Muted:     c("#615D7A", "#9B97B2"),
		Faint:     c("#8F8BAB", "#5E5A78"),
		Success:   c("#1A7F4B", "#3DDC97"),
		Warning:   c("#9A5B00", "#FFB454"),
		Danger:    c("#C0314B", "#FF6B81"),
		DangerBg:  c("#FFE1E7", "#471420"),
		Info:      c("#8639A8", "#D699FF"),
		Surface:   c("#F2F0FA", "#232136"),
		// Violet → magenta → amber, the app's spectrum character.
		SystemRamp: ramp([eqRows][2]string{
			{"#5A3FD6", "#7C5CFF"}, {"#5A3FD6", "#7C5CFF"},
			{"#7E3FC2", "#A55CFF"}, {"#7E3FC2", "#A55CFF"},
			{"#C23A93", "#F06BC8"}, {"#C23A93", "#F06BC8"},
			{"#B96A00", "#FFA53C"}, {"#8F7A00", "#FFE066"},
		}),
		MicRamp: ramp([eqRows][2]string{
			{"#B03A60", "#E0608A"}, {"#B03A60", "#E0608A"},
			{"#A238A2", "#D85FD8"}, {"#A238A2", "#D85FD8"},
			{"#C23A93", "#F06BC8"}, {"#C23A93", "#F06BC8"},
			{"#C4577F", "#FF9ED2"}, {"#A85E85", "#FFC9E8"},
		}),
	}
}

// Package styles, rebuilt by ApplyTheme. Every component in this package draws
// through these; nothing declares its own color literals.
var (
	headerStyle   lipgloss.Style
	subtleStyle   lipgloss.Style
	labelStyle    lipgloss.Style
	valueStyle    lipgloss.Style
	warnStyle     lipgloss.Style
	errStyle      lipgloss.Style
	okStyle       lipgloss.Style
	recStyle      lipgloss.Style
	boxTitleStyle lipgloss.Style
	borderStyle   lipgloss.Style // bare border color, for title-in-border splicing
	modalBoxStyle lipgloss.Style
	magentaStyle  lipgloss.Style
	dimStyle      lipgloss.Style // overlay background dimming

	subTabActiveStyle   lipgloss.Style
	subTabInactiveStyle lipgloss.Style
	tabActiveStyle      lipgloss.Style
	tabInactiveStyle    lipgloss.Style
	keyStyle            lipgloss.Style
	selRowStyle         lipgloss.Style

	badgeOKStyle      lipgloss.Style
	badgeWarnStyle    lipgloss.Style
	badgeMeetStyle    lipgloss.Style
	badgeNeutralStyle lipgloss.Style

	txDoneStyle      lipgloss.Style
	txPendingStyle   lipgloss.Style
	txActiveStyle    lipgloss.Style
	txActiveAltStyle lipgloss.Style
	txErrorStyle     lipgloss.Style
	footerBarStyle   lipgloss.Style

	configSidebarActiveStyle   lipgloss.Style
	configSidebarInactiveStyle lipgloss.Style

	systemEQStyles [eqRows]lipgloss.Style
	micEQStyles    [eqRows]lipgloss.Style
)

// ApplyTheme rebuilds the package styles for the given palette. The TUI calls
// it from Update when tea.BackgroundColorMsg reports the real terminal
// background; until then the dark palette from init applies. Update runs on
// the event-loop goroutine before the next View, so mutating the vars here is
// race-free, and View stays a pure function of the model per render.
func ApplyTheme(t Theme) {
	themeGen++
	headerStyle = lipgloss.NewStyle().Bold(true).Foreground(t.Accent)
	subtleStyle = lipgloss.NewStyle().Foreground(t.Faint)
	labelStyle = lipgloss.NewStyle().Foreground(t.Muted)
	valueStyle = lipgloss.NewStyle().Foreground(t.Text)
	warnStyle = lipgloss.NewStyle().Foreground(t.Warning)
	errStyle = lipgloss.NewStyle().Foreground(t.Danger)
	okStyle = lipgloss.NewStyle().Foreground(t.Success)
	recStyle = lipgloss.NewStyle().Bold(true).Foreground(t.Danger).Background(t.DangerBg)
	boxTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(t.Accent)
	borderStyle = lipgloss.NewStyle().Foreground(t.Border)
	modalBoxStyle = lipgloss.NewStyle().
		Border(lipgloss.ThickBorder()).
		BorderForeground(t.Accent).
		Background(t.Surface).
		Padding(1, 2)
	magentaStyle = lipgloss.NewStyle().Foreground(t.Info)
	dimStyle = lipgloss.NewStyle().Foreground(t.BorderDim)

	subTabActiveStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(t.AccentFg).
		Background(t.Accent).
		Padding(0, 1)
	subTabInactiveStyle = lipgloss.NewStyle().
		Foreground(t.Muted).
		Padding(0, 1)
	tabActiveStyle = subTabActiveStyle
	tabInactiveStyle = subTabInactiveStyle
	keyStyle = lipgloss.NewStyle().Bold(true).Foreground(t.Accent)
	selRowStyle = lipgloss.NewStyle().Foreground(t.AccentFg).Background(t.Accent)

	badgeOKStyle = lipgloss.NewStyle().Bold(true).Foreground(t.AccentFg).Background(t.Success)
	badgeWarnStyle = lipgloss.NewStyle().Bold(true).Foreground(t.AccentFg).Background(t.Warning)
	badgeMeetStyle = lipgloss.NewStyle().Bold(true).Foreground(t.AccentFg).Background(t.Info)
	badgeNeutralStyle = lipgloss.NewStyle().Foreground(t.Muted)

	txDoneStyle = lipgloss.NewStyle().Bold(true).Foreground(t.Success)
	txPendingStyle = lipgloss.NewStyle().Foreground(t.Faint)
	txActiveStyle = lipgloss.NewStyle().Bold(true).Foreground(t.Warning)
	txActiveAltStyle = lipgloss.NewStyle().Bold(true).Foreground(t.Info)
	txErrorStyle = lipgloss.NewStyle().Bold(true).Foreground(t.Danger)
	footerBarStyle = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(t.BorderDim)

	configSidebarActiveStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Accent).
		Bold(true).
		Foreground(t.Text).
		Padding(0, 1)
	configSidebarInactiveStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderDim).
		Foreground(t.Muted).
		Padding(0, 1)

	for i := 0; i < eqRows; i++ {
		systemEQStyles[i] = lipgloss.NewStyle().Foreground(t.SystemRamp[i])
		micEQStyles[i] = lipgloss.NewStyle().Foreground(t.MicRamp[i])
	}
}

func init() {
	ApplyTheme(DefaultTheme(true))
}

// themeGen counts ApplyTheme calls so render caches keyed on it invalidate
// when the palette swaps (e.g. the terminal reports a light background).
var themeGen uint64

// ThemeGen returns the current theme generation for cache keys.
func ThemeGen() uint64 { return themeGen }
