package config

import "fmt"

const (
	LevelPresetOff        = "off"
	LevelPresetResponsive = "responsive"
	LevelPresetBalanced   = "balanced"
	LevelPresetEconomy    = "economy"
	LevelPresetCustom     = "custom"
)

// LevelTuning is the parec + UI refresh timing for the Home level meter.
type LevelTuning struct {
	LatencyMsec     int
	ProcessTimeMsec int
	UITickMS        int
}

// LevelPresetOptions are labels shown in the config enum modal.
func LevelPresetOptions() []string {
	return []string{
		LevelPresetLabel(LevelPresetOff),
		LevelPresetLabel(LevelPresetResponsive),
		LevelPresetLabel(LevelPresetBalanced),
		LevelPresetLabel(LevelPresetEconomy),
		LevelPresetLabel(LevelPresetCustom),
	}
}

// LevelPresetLabel returns the user-facing preset name.
func LevelPresetLabel(id string) string {
	switch id {
	case LevelPresetOff:
		return "off (disabled)"
	case LevelPresetResponsive:
		return "responsive"
	case LevelPresetBalanced:
		return "balanced"
	case LevelPresetEconomy:
		return "economy"
	case LevelPresetCustom:
		return "custom"
	default:
		return "custom"
	}
}

// LevelPresetID parses a modal option label back to a preset id.
func LevelPresetID(label string) string {
	for _, id := range []string{LevelPresetOff, LevelPresetResponsive, LevelPresetBalanced, LevelPresetEconomy, LevelPresetCustom} {
		if LevelPresetLabel(id) == label {
			return id
		}
	}
	if label == LevelPresetOff || label == LevelPresetResponsive || label == LevelPresetBalanced || label == LevelPresetEconomy || label == LevelPresetCustom {
		return label
	}
	return LevelPresetCustom
}

// LevelTuningForPreset returns fixed tuning for a built-in preset.
func LevelTuningForPreset(id string) (LevelTuning, bool) {
	switch id {
	case LevelPresetResponsive:
		// Measured ~10% CPU on typical Linux + PipeWire setups.
		return LevelTuning{LatencyMsec: 50, ProcessTimeMsec: 20, UITickMS: 33}, true
	case LevelPresetBalanced:
		// Target ~5% CPU: moderate parec buffering + ~12 FPS UI.
		return LevelTuning{LatencyMsec: 500, ProcessTimeMsec: 200, UITickMS: 80}, true
	case LevelPresetEconomy:
		// Target ~1% CPU: large parec chunks + ~3 FPS UI.
		return LevelTuning{LatencyMsec: 1000, ProcessTimeMsec: 400, UITickMS: 300}, true
	default:
		return LevelTuning{}, false
	}
}

// InferLevelPreset matches current tuning to a preset, or custom.
func InferLevelPreset(latency, process, ui int) string {
	for _, id := range []string{LevelPresetResponsive, LevelPresetBalanced, LevelPresetEconomy} {
		t, ok := LevelTuningForPreset(id)
		if ok && t.LatencyMsec == latency && t.ProcessTimeMsec == process && t.UITickMS == ui {
			return id
		}
	}
	return LevelPresetCustom
}

// LevelMeterEnabled reports whether the Home level meter should run and render.
func LevelMeterEnabled(cfg Config) bool {
	return cfg.Audio.LevelPreset != LevelPresetOff
}

// ApplyLevelPreset writes preset tuning into cfg and sets LevelPreset.
func ApplyLevelPreset(cfg *Config, id string) error {
	if id == LevelPresetOff {
		cfg.Audio.LevelPreset = LevelPresetOff
		return nil
	}
	if id == LevelPresetCustom {
		cfg.Audio.LevelPreset = LevelPresetCustom
		return nil
	}
	t, ok := LevelTuningForPreset(id)
	if !ok {
		return fmt.Errorf("unknown level preset %q", id)
	}
	cfg.Audio.LevelPreset = id
	cfg.Audio.LevelLatencyMsec = t.LatencyMsec
	cfg.Audio.LevelProcessTimeMsec = t.ProcessTimeMsec
	cfg.Audio.LevelUITickMS = t.UITickMS
	return nil
}

// MarkLevelPresetCustom flags manual tuning edits.
func MarkLevelPresetCustom(cfg *Config) {
	cfg.Audio.LevelPreset = LevelPresetCustom
}
