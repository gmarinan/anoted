package setup

import (
	"strings"
	"testing"

	"anoted/internal/config"
	"anoted/internal/platform"
)

func TestNeedsSetupIncomplete(t *testing.T) {
	cfg := config.Default()
	plat := platform.Info{OS: platform.OSLinux, Session: "x11"}
	if !NeedsSetup(cfg, plat) {
		t.Fatal("expected needs setup when not completed")
	}
}

func TestNeedsSetupCompleted(t *testing.T) {
	cfg := config.Default()
	cfg.SetupCompleted = true
	plat := platform.Info{OS: platform.OSLinux, Session: "x11"}
	if NeedsSetup(cfg, plat) {
		t.Fatal("should not need setup when completed")
	}
}

func TestChooseDetectionModeDefault(t *testing.T) {
	got := chooseDetectionMode(strings.NewReader("\n"), &strings.Builder{}, platform.Info{Session: "x11"})
	if got != DetMic {
		t.Fatalf("got %q", got)
	}
}
