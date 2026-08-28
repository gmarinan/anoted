package tui

import "testing"

// View must be a pure function of the Model. It used to stat several paths per
// session row, read the selected transcript, and sweep $PATH for file managers
// — up to thirty times a second while the level meter ran, which stalls the
// render loop on a network home directory or a spun-down disk.
//
// Rendering the same Model twice must produce byte-identical output and must
// not depend on anything outside it.
func TestViewIsPureAndRepeatable(t *testing.T) {
	m := Model{
		screen:   ScreenMain,
		width:    120,
		height:   40,
		sessions: testSessions(5),
		scroll:   newSessionScroll(),
	}
	first := m.View()
	second := m.View()
	if first.Content != second.Content {
		t.Fatal("View is not a pure function of the Model")
	}
}
