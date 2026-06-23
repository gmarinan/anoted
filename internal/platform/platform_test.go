package platform

import (
	"testing"
	"time"
)

func TestInfoSubtitle(t *testing.T) {
	tests := []struct {
		info Info
		want string
	}{
		{Info{OS: OSLinux}, "Linux · PipeWire"},
		{Info{OS: OSLinux, IsWSL2: true}, "WSL2 · WASAPI"},
		{Info{OS: OSWindows}, "Windows · WASAPI"},
		{Info{OS: OSUnknown}, "Unknown"},
	}
	for _, tt := range tests {
		if got := tt.info.Subtitle(); got != tt.want {
			t.Fatalf("%+v: got %q want %q", tt.info, got, tt.want)
		}
	}
}

func TestInfoWindowSizePollInterval(t *testing.T) {
	if got := (Info{OS: OSWindows}).WindowSizePollInterval(); got != 200*time.Millisecond {
		t.Fatalf("windows: got %v want 200ms", got)
	}
	if got := (Info{OS: OSLinux}).WindowSizePollInterval(); got != 0 {
		t.Fatalf("linux: got %v want 0", got)
	}
}

func TestInfoClearScreenOnResize(t *testing.T) {
	if !(Info{OS: OSWindows}).ClearScreenOnResize() {
		t.Fatal("windows should clear on resize")
	}
	if (Info{OS: OSLinux}).ClearScreenOnResize() {
		t.Fatal("linux should not clear on resize")
	}
}
