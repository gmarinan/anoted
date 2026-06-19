//go:build linux

package audio

import (
	"os/exec"
	"strings"
	"testing"
)

func TestLinuxProviderList(t *testing.T) {
	if _, err := exec.LookPath("pactl"); err != nil {
		t.Skip("pactl not available")
	}
	p := linuxProvider{}
	cat, err := p.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(cat.Outputs) < 2 {
		t.Fatalf("expected outputs including auto, got %d", len(cat.Outputs))
	}
	if len(cat.Microphones) < 2 {
		t.Fatalf("expected mics including auto, got %d", len(cat.Microphones))
	}
	for _, o := range cat.Outputs {
		if o.ID == AutoID {
			continue
		}
		if o.NodeID == "" || o.Name == "" || o.State == "" {
			t.Fatalf("incomplete output device: %+v", o)
		}
		if !strings.HasPrefix(o.Name, "alsa_output.") {
			t.Fatalf("expected sink name, got %q", o.Name)
		}
		if !strings.HasSuffix(o.ID, ".monitor") {
			t.Fatalf("output ID should be monitor, got %q", o.ID)
		}
	}
}

func TestLinuxProviderResolve(t *testing.T) {
	if _, err := exec.LookPath("pactl"); err != nil {
		t.Skip("pactl not available")
	}
	p := linuxProvider{}
	sys, mic, err := p.Resolve("", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(sys, ".monitor") {
		t.Fatalf("unexpected monitor: %q", sys)
	}
	if mic == "" {
		t.Fatal("expected mic name")
	}
}

func TestListSinkLinkedApps(t *testing.T) {
	if _, err := exec.LookPath("pactl"); err != nil {
		t.Skip("pactl not available")
	}
	linked := listSinkLinkedApps()
	// map may be empty when nothing is playing; just ensure no panic
	_ = linked
}
