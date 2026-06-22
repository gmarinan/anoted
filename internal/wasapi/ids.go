//go:build windows

package wasapi

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/gen2brain/malgo"
)

const (
	prefixLoopback = "wasapi:loopback:"
	prefixCapture  = "wasapi:capture:"
)

// LoopbackID encodes a playback device ID for system audio capture.
func LoopbackID(id malgo.DeviceID) string {
	return prefixLoopback + id.String()
}

// CaptureID encodes a capture device ID for microphone input.
func CaptureID(id malgo.DeviceID) string {
	return prefixCapture + id.String()
}

// ParseLoopbackID returns the malgo device ID from a stored loopback ID.
func ParseLoopbackID(stored string) (malgo.DeviceID, error) {
	return parseDeviceID(stored, prefixLoopback)
}

// ParseCaptureID returns the malgo device ID from a stored capture ID.
func ParseCaptureID(stored string) (malgo.DeviceID, error) {
	return parseDeviceID(stored, prefixCapture)
}

func parseDeviceID(stored, prefix string) (malgo.DeviceID, error) {
	if stored == "" {
		return malgo.DeviceID{}, nil
	}
	if !strings.HasPrefix(stored, prefix) {
		return malgo.DeviceID{}, fmt.Errorf("invalid device id %q: want prefix %s", stored, prefix)
	}
	hexID := strings.TrimPrefix(stored, prefix)
	raw, err := hex.DecodeString(hexID)
	if err != nil {
		return malgo.DeviceID{}, fmt.Errorf("parse device id %q: %w", stored, err)
	}
	var id malgo.DeviceID
	copy(id[:], raw)
	return id, nil
}
