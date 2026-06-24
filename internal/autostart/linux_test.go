//go:build linux

package autostart

import (
	"strings"
	"testing"
)

func TestDesktopExecLineQuotesSpaces(t *testing.T) {
	got := desktopExecLine("/opt/my apps/anoted", []string{"watch"})
	want := `"/opt/my apps/anoted" watch`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRenderDesktop(t *testing.T) {
	content := renderDesktop(Entry{Exec: "/usr/bin/anoted", Args: []string{"watch"}})
	for _, want := range []string{
		"Type=Application",
		"Exec=/usr/bin/anoted watch",
		"Terminal=true",
		"X-GNOME-Autostart-enabled=true",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("missing %q in:\n%s", want, content)
		}
	}
}

func TestRenderDesktopWithTerminalWrapper(t *testing.T) {
	content := renderDesktop(Entry{
		Exec:            "/usr/bin/anoted",
		Args:            []string{"watch"},
		WMClass:         "anoted",
		TerminalCommand: []string{"alacritty", "--class", "anoted", "-e"},
	})
	for _, want := range []string{
		`Exec=alacritty --class anoted -e /usr/bin/anoted watch`,
		"Terminal=false",
		"StartupWMClass=anoted",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("missing %q in:\n%s", want, content)
		}
	}
}
