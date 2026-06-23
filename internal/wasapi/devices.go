//go:build windows

package wasapi

import (
	"fmt"

	"github.com/gen2brain/malgo"
)

// Endpoint describes a WASAPI audio endpoint.
type Endpoint struct {
	ID        string
	Name      string
	IsDefault bool
	Format    string
}

// ListLoopback returns playback devices usable for loopback capture.
func ListLoopback() ([]Endpoint, error) {
	ctx, err := Context()
	if err != nil {
		return nil, err
	}
	devs, err := ctx.Devices(malgo.Playback)
	if err != nil {
		return nil, fmt.Errorf("list playback devices: %w", err)
	}
	out := make([]Endpoint, 0, len(devs))
	for i := range devs {
		d := &devs[i]
		out = append(out, Endpoint{
			ID:        LoopbackID(d.ID),
			Name:      d.Name(),
			IsDefault: d.IsDefault != 0,
			Format:    formatSummary(d),
		})
	}
	return out, nil
}

// ListCapture returns microphone / capture endpoints.
func ListCapture() ([]Endpoint, error) {
	ctx, err := Context()
	if err != nil {
		return nil, err
	}
	devs, err := ctx.Devices(malgo.Capture)
	if err != nil {
		return nil, fmt.Errorf("list capture devices: %w", err)
	}
	out := make([]Endpoint, 0, len(devs))
	for i := range devs {
		d := &devs[i]
		out = append(out, Endpoint{
			ID:        CaptureID(d.ID),
			Name:      d.Name(),
			IsDefault: d.IsDefault != 0,
			Format:    formatSummary(d),
		})
	}
	return out, nil
}

// ResolveLoopback maps a stored ID (empty = default) to a malgo playback device ID.
func ResolveLoopback(stored string) (malgo.DeviceID, error) {
	if stored == "" {
		return defaultPlaybackID()
	}
	return ParseLoopbackID(stored)
}

// ResolveCapture maps a stored ID (empty = default) to a malgo capture device ID.
func ResolveCapture(stored string) (malgo.DeviceID, error) {
	if stored == "" {
		return defaultCaptureID()
	}
	return ParseCaptureID(stored)
}

func defaultPlaybackID() (malgo.DeviceID, error) {
	devs, err := ListLoopback()
	if err != nil {
		return malgo.DeviceID{}, err
	}
	for _, d := range devs {
		if d.IsDefault {
			return ParseLoopbackID(d.ID)
		}
	}
	if len(devs) == 0 {
		return malgo.DeviceID{}, fmt.Errorf("no playback devices found")
	}
	return ParseLoopbackID(devs[0].ID)
}

func defaultCaptureID() (malgo.DeviceID, error) {
	devs, err := ListCapture()
	if err != nil {
		return malgo.DeviceID{}, err
	}
	for _, d := range devs {
		if d.IsDefault {
			return ParseCaptureID(d.ID)
		}
	}
	if len(devs) == 0 {
		return malgo.DeviceID{}, fmt.Errorf("no capture devices found")
	}
	return ParseCaptureID(devs[0].ID)
}

// LoopbackName returns the display name for a stored loopback ID.
func LoopbackName(stored string) (string, error) {
	if stored == "" {
		storedID, err := defaultPlaybackID()
		if err != nil {
			return "", err
		}
		stored = LoopbackID(storedID)
	}
	want, err := ParseLoopbackID(stored)
	if err != nil {
		return "", err
	}
	devs, err := ListLoopback()
	if err != nil {
		return "", err
	}
	for _, d := range devs {
		id, err := ParseLoopbackID(d.ID)
		if err != nil {
			continue
		}
		if id == want {
			return d.Name, nil
		}
	}
	return stored, nil
}

// NativeSampleRate returns the preferred sample rate for a device endpoint.
func NativeSampleRate(id malgo.DeviceID, kind malgo.DeviceType) (uint32, error) {
	ctx, err := Context()
	if err != nil {
		return 0, err
	}
	info, err := ctx.DeviceInfo(kind, id, malgo.Shared)
	if err != nil {
		return 0, fmt.Errorf("device info: %w", err)
	}
	if info.FormatCount == 0 {
		return 48000, nil
	}
	for _, prefer := range []uint32{48000, 44100, 96000, 88200} {
		for _, f := range info.Formats {
			if f.SampleRate == prefer {
				return prefer, nil
			}
		}
	}
	return info.Formats[0].SampleRate, nil
}

func formatSummary(d *malgo.DeviceInfo) string {
	if d == nil || d.FormatCount == 0 {
		return ""
	}
	f := d.Formats[0]
	return fmt.Sprintf("%s %dch %dHz", formatName(f.Format), f.Channels, f.SampleRate)
}

func formatName(f malgo.FormatType) string {
	switch f {
	case malgo.FormatS16:
		return "s16"
	case malgo.FormatF32:
		return "f32"
	case malgo.FormatS32:
		return "s32"
	case malgo.FormatS24:
		return "s24"
	case malgo.FormatU8:
		return "u8"
	default:
		return "unknown"
	}
}
