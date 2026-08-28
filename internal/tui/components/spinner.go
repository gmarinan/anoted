package components

// spinnerFrames is a braille spinner. Every glyph is single-width, so it does
// not disturb the box widths the way an emoji would.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Spinner returns the frame for the given tick.
//
// Long installs used to render a static "Installing dependencies…" with no
// other sign of life. pip can go minutes between log lines while resolving or
// downloading, and with no animation and no repaint the app looked hung exactly
// when the user was most likely to kill it.
func Spinner(frame uint64) string {
	return spinnerFrames[int(frame%uint64(len(spinnerFrames)))]
}
