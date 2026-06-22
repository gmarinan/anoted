package config

import "testing"

func TestLevelTuningForPreset(t *testing.T) {
	tuning, ok := LevelTuningForPreset(LevelPresetResponsive)
	if !ok || tuning.UITickMS != 33 {
		t.Fatalf("responsive: %+v ok=%v", tuning, ok)
	}
}

func TestInferLevelPreset(t *testing.T) {
	if got := InferLevelPreset(50, 20, 33); got != LevelPresetResponsive {
		t.Fatalf("got %q", got)
	}
	if got := InferLevelPreset(500, 200, 100); got != LevelPresetCustom {
		t.Fatalf("got %q want custom", got)
	}
}

func TestLevelMeterEnabled(t *testing.T) {
	cfg := Default()
	if !LevelMeterEnabled(cfg) {
		t.Fatal("default should enable meter")
	}
	cfg.Audio.LevelPreset = LevelPresetOff
	if LevelMeterEnabled(cfg) {
		t.Fatal("off should disable meter")
	}
}

func TestApplyLevelPresetOff(t *testing.T) {
	cfg := Default()
	if err := ApplyLevelPreset(&cfg, LevelPresetOff); err != nil {
		t.Fatal(err)
	}
	if cfg.Audio.LevelPreset != LevelPresetOff {
		t.Fatalf("preset %q", cfg.Audio.LevelPreset)
	}
}

func TestApplyLevelPreset(t *testing.T) {
	cfg := Default()
	if err := ApplyLevelPreset(&cfg, LevelPresetEconomy); err != nil {
		t.Fatal(err)
	}
	if cfg.Audio.LevelPreset != LevelPresetEconomy {
		t.Fatalf("preset %q", cfg.Audio.LevelPreset)
	}
	if cfg.Audio.LevelUITickMS != 300 {
		t.Fatalf("ui tick %d", cfg.Audio.LevelUITickMS)
	}
}
