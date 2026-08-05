//go:build linux

package autostart

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnableDisableLinux(t *testing.T) {
	if os.Getenv("HOME") == "" {
		t.Skip("HOME not set")
	}
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	entry := Entry{Exec: "/usr/bin/anoted", Args: []string{"watch"}}
	if err := Enable(entry); err != nil {
		t.Fatal(err)
	}
	if !Enabled() {
		t.Fatal("expected enabled")
	}
	path, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(string(data), "/usr/bin/anoted watch") {
		t.Fatalf("unexpected desktop entry:\n%s", data)
	}
	if err := Disable(); err != nil {
		t.Fatal(err)
	}
	if Enabled() {
		t.Fatal("expected disabled")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestAutostartDirUsesXDGConfigHome(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)
	dir, err := autostartDir()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "autostart")
	if dir != want {
		t.Fatalf("got %q want %q", dir, want)
	}
}
