package tui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"anoted/internal/config"
	"anoted/internal/retention"
	"anoted/internal/session"
)

func TestStorageSectionOffersRetentionControls(t *testing.T) {
	if configSectionCount != len(configSectionLabels) {
		t.Fatalf("sections = %d, labels = %d", configSectionCount, len(configSectionLabels))
	}
	if configSectionLabels[6] != "Storage" {
		t.Fatalf("section = %q", configSectionLabels[6])
	}
	fields := cfgFieldsWithEnv(6, envFacts{
		AudioUsage:   "1.0 KB",
		AudioPreview: "off — set keep_days or keep_recordings",
	})
	byLabel := map[string]cfgField{}
	for _, f := range fields {
		byLabel[f.label] = f
	}
	for _, want := range []string{
		"audio_on_disk", "auto_delete", "keep_days", "keep_recordings", "protect_untranscribed", "would_free",
	} {
		if _, ok := byLabel[want]; !ok {
			t.Fatalf("missing %s", want)
		}
	}
	cfg := config.Default()
	if byLabel["auto_delete"].get(cfg) != "false" {
		t.Fatal(byLabel["auto_delete"].get(cfg))
	}
	if byLabel["protect_untranscribed"].get(cfg) != "true" {
		t.Fatal(byLabel["protect_untranscribed"].get(cfg))
	}
	if byLabel["audio_on_disk"].get(cfg) != "1.0 KB" {
		t.Fatal(byLabel["audio_on_disk"].get(cfg))
	}
	if err := byLabel["keep_days"].set(&cfg, "30"); err != nil {
		t.Fatal(err)
	}
	if cfg.Retention.KeepDays != 30 {
		t.Fatalf("keep_days = %d", cfg.Retention.KeepDays)
	}
	if err := byLabel["keep_recordings"].set(&cfg, "0"); err != nil {
		t.Fatal(err)
	}
	if err := byLabel["protect_untranscribed"].set(&cfg, "false"); err != nil {
		t.Fatal(err)
	}
	if cfg.Retention.ProtectsUntranscribed() {
		t.Fatal("protect_untranscribed should turn off")
	}
}

func TestRetentionFollowUpRespectsCurrentPolicy(t *testing.T) {
	dir := t.TempDir()
	wav := filepath.Join(dir, "recording.wav")
	if err := os.WriteFile(wav, []byte("abcd"), 0o600); err != nil {
		t.Fatal(err)
	}
	transcript := filepath.Join(dir, "transcript.txt")
	if err := os.WriteFile(transcript, []byte("notes"), 0o600); err != nil {
		t.Fatal(err)
	}
	plan := retention.Plan{Remove: []retention.Removal{{
		Dir: dir, Path: wav, Bytes: 4, Reason: "older than 30 days",
	}}}
	pol := retention.Policy{Enabled: true, KeepDays: 30, ProtectUntranscribed: true}

	off := Model{deps: Deps{Config: config.Default()}}
	if cmd := off.retentionFollowUp(pol, plan); cmd != nil {
		t.Fatal("auto_delete off must not schedule deletion")
	}
	if _, err := os.Stat(wav); err != nil {
		t.Fatal(err)
	}

	on := off
	on.deps.Config.Retention.AutoDelete = true
	on.deps.Config.Retention.KeepDays = 30
	cmd := on.retentionFollowUp(pol, plan)
	if cmd == nil {
		t.Fatal("matching enabled policy must schedule deletion")
	}
	done, ok := cmd().(retentionDoneMsg)
	if !ok {
		t.Fatalf("msg type %T", cmd())
	}
	if done.err != nil || done.removed != 1 || done.freed != 4 {
		t.Fatalf("%+v", done)
	}
	if _, err := os.Stat(wav); !os.IsNotExist(err) {
		t.Fatal("recording.wav should have been removed")
	}
	if _, err := os.Stat(transcript); err != nil {
		t.Fatal("transcript must stay")
	}
}

func TestLibraryItemsMarksLiveAndOutsideAudio(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "sess")
	if err := os.Mkdir(inside, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inside, "recording.wav"), []byte("abc"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inside, "transcript.txt"), []byte("hi"), 0o600); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "recording.wav"), []byte("xyzxyz"), 0o600); err != nil {
		t.Fatal(err)
	}

	ended := time.Now().Add(-48 * time.Hour)
	recs := []session.Record{
		{Dir: inside, StartedAt: ended.Add(-time.Hour), EndedAt: ended},
		{Dir: outside, StartedAt: ended.Add(-time.Hour), EndedAt: ended},
		{Dir: inside, StartedAt: ended.Add(-time.Hour), EndedAt: ended},
	}
	items := libraryItems(root, recs, config.TranscriptionConfig{}, map[string]struct{}{inside: {}})
	if len(items) != 2 {
		t.Fatalf("items = %d, duplicate session dirs must collapse", len(items))
	}

	var live, out retention.Item
	for _, it := range items {
		switch it.Dir {
		case inside:
			live = it
		case outside:
			out = it
		}
	}
	if !live.InUse || !live.HasAudio || live.AudioBytes != 3 || !live.HasTranscript || live.Outside {
		t.Fatalf("live %+v", live)
	}
	if !out.Outside || !out.HasAudio || out.AudioBytes != 6 || out.HasTranscript {
		t.Fatalf("outside %+v", out)
	}

	plan := retention.BuildPlan(time.Now(), retention.Policy{Enabled: true, KeepDays: 1}, items)
	if len(plan.Remove) != 0 {
		t.Fatalf("in-use and outside audio must not be deleted: %+v", plan.Remove)
	}
	usage := retention.Summarize(items)
	if usage.Files != 2 || usage.Bytes != 9 {
		t.Fatalf("usage %+v", usage)
	}
}
