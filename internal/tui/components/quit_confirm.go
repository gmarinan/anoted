package components

import "strings"

// QuitConfirmView renders the quit-while-busy confirmation modal.
type QuitConfirmView struct {
	Reasons []string
	Cursor  int
	Width   int
	Height  int
}

func (v QuitConfirmView) Overlay(base string) string {
	if len(v.Reasons) == 0 {
		return base
	}
	h := v.Height - 8
	if h < 12 {
		h = 12
	}
	return FloatCenter(base, v.renderModal(), v.Width, h)
}

func (v QuitConfirmView) renderModal() string {
	var lines []string
	lines = append(lines, warnStyle.Render("Quit anoted?"))
	lines = append(lines, "")
	lines = append(lines, "The following is still in progress:")
	for _, r := range v.Reasons {
		lines = append(lines, "  • "+r)
	}
	lines = append(lines, "")
	lines = append(lines, errStyle.Render("Unsaved progress may be lost."))
	lines = append(lines, "")

	choices := []string{"No, stay", "Yes, quit"}
	for i, label := range choices {
		marker := "  "
		if i == v.Cursor {
			marker = "> "
		}
		line := marker + label
		if i == v.Cursor {
			line = valueStyle.Bold(true).Render(line)
		} else {
			line = valueStyle.Render(line)
		}
		lines = append(lines, line)
	}
	lines = append(lines, "")
	lines = append(lines, subtleStyle.Render("↑↓ choose · Enter apply · Esc cancel"))

	maxW := v.Width - 10
	if maxW < 40 {
		maxW = 40
	}
	if maxW > 58 {
		maxW = 58
	}
	return PickerModal("Confirm quit", strings.Join(lines, "\n"), maxW)
}

// QuitConfirmFooter returns footer hints while the quit modal is open.
func QuitConfirmFooter() string {
	return JoinFooter(
		FooterHint("↑↓", "choose"),
		FooterHint("Enter", "apply"),
		FooterHint("Esc", "cancel"),
	)
}

// FormatQuitReasons builds human-readable reason lines for the modal.
func FormatQuitReasons(recording, transcribing, installing bool) []string {
	var reasons []string
	if recording {
		reasons = append(reasons, "Audio recording")
	}
	if transcribing {
		reasons = append(reasons, "Whisper transcription")
	}
	if installing {
		reasons = append(reasons, "Background install")
	}
	return reasons
}
