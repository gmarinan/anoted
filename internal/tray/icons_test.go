package tray

import "testing"

func TestIconForStates(t *testing.T) {
	if len(iconFor(StateMonitoring)) == 0 {
		t.Fatal("monitoring icon empty")
	}
	if len(iconFor(StateRecording)) == 0 {
		t.Fatal("recording icon empty")
	}
	if string(iconFor(StateMonitoring)) == string(iconFor(StateRecording)) {
		t.Fatal("icons should differ between states")
	}
}

func TestTooltipForStates(t *testing.T) {
	if tooltipFor(StateRecording) == tooltipFor(StateMonitoring) {
		t.Fatal("tooltips should differ")
	}
}

func TestNoopIndicator(t *testing.T) {
	var n noopIndicator
	if err := n.Start(); err != nil {
		t.Fatal(err)
	}
	n.SetState(StateRecording)
	n.Stop()
}
