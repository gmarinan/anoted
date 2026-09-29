package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRetentionDefaultsProtectAudio(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "missing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Retention.AutoDelete {
		t.Fatal("auto_delete must stay off until the user opts in")
	}
	if !cfg.Retention.ProtectsUntranscribed() {
		t.Fatal("missing protect_untranscribed must keep audio that has no transcript")
	}
	if cfg.Retention.KeepDays != 0 || cfg.Retention.KeepRecordings != 0 {
		t.Fatalf("caps = %d days, %d recordings; zero means no limit", cfg.Retention.KeepDays, cfg.Retention.KeepRecordings)
	}
}

func TestRetentionExplicitOptOutIsKept(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	body := "retention:\n  protect_untranscribed: false\n  keep_days: 14\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Retention.ProtectsUntranscribed() {
		t.Fatal("an explicit false must not be rewritten to true")
	}
	if cfg.Retention.KeepDays != 14 {
		t.Fatalf("keep_days = %d", cfg.Retention.KeepDays)
	}
}

func TestRetentionRejectsNegativeCaps(t *testing.T) {
	cfg := Default()
	cfg.Retention.KeepDays = -1
	cfg.Retention.KeepRecordings = -3
	err := cfg.Validate()
	if err == nil {
		t.Fatal("negative retention caps must be rejected")
	}
	if !strings.Contains(err.Error(), "keep_days") || !strings.Contains(err.Error(), "keep_recordings") {
		t.Fatal(err)
	}
}
