package platform

import "testing"

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
