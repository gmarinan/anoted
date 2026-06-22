package tui

import (
	"testing"

	"anoted/internal/config"
)

func TestAppendConfigInputRuneIntField(t *testing.T) {
	m := Model{
		configEditing:  true,
		configSection:  1, // Audio
		configCursor:   0, // sample_rate (fieldInt)
		deps:           Deps{Config: config.Default()},
	}
	m = m.appendConfigInputRune('1')
	m = m.appendConfigInputRune('0')
	m = m.appendConfigInputRune('0')
	if m.configInput != "100" {
		t.Fatalf("got %q want 100", m.configInput)
	}
	m = m.appendConfigInputRune('x')
	if m.configInput != "100" {
		t.Fatalf("non-digit should be ignored, got %q", m.configInput)
	}
}

func TestConfigAbsorbsTabKeysWhileEditing(t *testing.T) {
	m := Model{configEditing: true}
	if !m.configAbsorbsKeys() {
		t.Fatal("editing should absorb tab switch keys")
	}
	if m.isTabSwitchKey("1") && m.configAbsorbsKeys() {
		// nav.go must check configAbsorbsKeys before handleTabSwitch
	}
}
