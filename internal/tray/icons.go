package tray

import _ "embed"

//go:embed icons/monitoring.png
var monitoringIcon []byte

//go:embed icons/recording.png
var recordingIcon []byte

func iconFor(state State) []byte {
	if state == StateRecording {
		return recordingIcon
	}
	return monitoringIcon
}

func tooltipFor(state State) string {
	if state == StateRecording {
		return "anoted — RECORDING"
	}
	return "anoted — watching"
}
