//go:build windows

package doctor

import (
	"testing"

	"anoted/internal/audio"
	"anoted/internal/config"
)

func TestFriendlyDeviceNameAuto(t *testing.T) {
	if got := friendlyDeviceName("", audio.Catalog{}, false); got != "(auto)" {
		t.Fatalf("got %q", got)
	}
}

func TestFriendlyDeviceNameFromCatalog(t *testing.T) {
	cat := audio.Catalog{
		Outputs: []audio.Device{{ID: "wasapi:loopback:abcd", Name: "Speakers (Realtek)"}},
	}
	if got := friendlyDeviceName("wasapi:loopback:abcd", cat, false); got != "Speakers (Realtek)" {
		t.Fatalf("got %q", got)
	}
}

func TestFriendlyDeviceNameFallback(t *testing.T) {
	got := friendlyDeviceName("wasapi:loopback:deadbeef", audio.Catalog{}, false)
	if got == "" || got == "wasapi:loopback:deadbeef" {
		t.Fatalf("expected short label, got %q", got)
	}
}

func TestAudioDeviceChecksOmitsRedundantConfigured(t *testing.T) {
	cfg := configWithAudio("", "")
	checks := audioDeviceChecks(cfg)
	for _, c := range checks {
		if c.Name == "configured_system_monitor" || c.Name == "configured_microphone" {
			t.Fatalf("unexpected redundant check %s", c.Name)
		}
	}
}

func configWithAudio(sys, mic string) config.Config {
	return config.Config{
		Audio: config.AudioConfig{
			SystemMonitor: sys,
			Microphone:    mic,
		},
	}
}
