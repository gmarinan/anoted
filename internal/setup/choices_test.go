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

func TestDetectionChoicesWindowsDefault(t *testing.T) {
	plat := platform.Info{OS: platform.OSWindows, Session: "windows"}
	choices := DetectionChoices(plat)
	if len(choices) < 2 {
		t.Fatalf("expected choices, got %d", len(choices))
	}
	if choices[0].Mode != DetMic || !choices[0].Recommended {
		t.Fatalf("first choice should be recommended mic, got %+v", choices[0])
	}
	if DefaultDetectionMode(plat) != DetMic {
		t.Fatalf("default mode should be mic")
	}
}

func TestChooseDetectionModeDefaultLinux(t *testing.T) {
	got := chooseDetectionMode(strings.NewReader("\n"), &strings.Builder{}, platform.Info{Session: "x11"})
	if got != DetMic {
		t.Fatalf("got %q", got)
	}
}
