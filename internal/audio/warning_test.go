//go:build linux

package audio

import (
	"os/exec"
	"strings"
	"testing"
)

func TestMonitorWarningNonDefaultMonitor(t *testing.T) {
	if _, err := exec.LookPath("pactl"); err != nil {
		t.Skip("pactl not available")
	}
	p := linuxProvider{}
	warn := p.MonitorWarning("alsa_output.pci-0000_29_00.1.hdmi-stereo.monitor")
	if warn == "" {
		t.Fatal("expected warning for non-default hdmi monitor")
	}
	if !strings.Contains(warn, "SUSPENDED") && !strings.Contains(warn, "not the default") {
		t.Fatalf("unexpected warning: %q", warn)
	}
}

func TestMonitorWarningAutoEmpty(t *testing.T) {
	p := linuxProvider{}
	if warn := p.MonitorWarning(""); warn != "" {
		t.Fatalf("expected no warning for auto, got %q", warn)
	}
}
