package retention

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBuildPlanKeepsNewestAndDropsOlderAudio(t *testing.T) {
	now := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	items := []Item{
		audio("new", now.Add(-time.Hour), 100, true),
		audio("mid", now.Add(-48*time.Hour), 200, true),
		audio("old", now.Add(-10*24*time.Hour), 400, true),
	}

	plan := BuildPlan(now, Policy{KeepRecordings: 2}, items)
	if got := paths(plan); len(got) != 1 || got[0] != AudioPath("old") {
		t.Fatalf("removed %v, want only old", got)
	}
	if plan.Protected != 0 {
		t.Fatalf("protected = %d", plan.Protected)
	}
}

func TestBuildPlanAgeLimit(t *testing.T) {
	now := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	items := []Item{
		audio("fresh", now.Add(-29*24*time.Hour), 10, true),
		audio("due", now.Add(-30*24*time.Hour), 10, true),
		audio("older", now.Add(-31*24*time.Hour), 10, true),
	}

	plan := BuildPlan(now, Policy{KeepDays: 30}, items)
	got := paths(plan)
	if len(got) != 2 {
		t.Fatalf("removed %d files, want the two that are at least 30 days old", len(got))
	}
	if !contains(got, AudioPath("due")) || !contains(got, AudioPath("older")) {
		t.Fatalf("removed %v", got)
	}
}

func TestBuildPlanEitherLimitDeletes(t *testing.T) {
	now := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	// Newest is inside the count but older than the age cap, so the age cap wins.
	items := []Item{
		audio("ancient-newest", now.Add(-40*24*time.Hour), 50, true),
		audio("ancient-next", now.Add(-41*24*time.Hour), 50, true),
	}

	plan := BuildPlan(now, Policy{KeepDays: 30, KeepRecordings: 10}, items)
	if len(plan.Remove) != 2 {
		t.Fatalf("removed %d, want both — age applies even inside the count", len(plan.Remove))
	}
}

func TestBuildPlanProtectsUntranscribed(t *testing.T) {
	now := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	items := []Item{
		audio("old-notes", now.Add(-40*24*time.Hour), 80, true),
		audio("old-raw", now.Add(-50*24*time.Hour), 800, false),
	}

	plan := BuildPlan(now, Policy{KeepDays: 30, ProtectUntranscribed: true}, items)
	if len(plan.Remove) != 1 || plan.Remove[0].Dir != "old-notes" {
		t.Fatalf("removed %+v, want only the transcribed audio", plan.Remove)
	}
	if plan.Protected != 1 || plan.ProtectedBytes != 800 {
		t.Fatalf("protected = %d bytes %d, want the untranscribed wav spared", plan.Protected, plan.ProtectedBytes)
	}
}

func TestBuildPlanUntranscribedStillOccupiesASlot(t *testing.T) {
	now := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	items := []Item{
		audio("raw-new", now.Add(-time.Hour), 10, false),
		audio("notes-old", now.Add(-2*time.Hour), 10, true),
	}

	plan := BuildPlan(now, Policy{KeepRecordings: 1, ProtectUntranscribed: true}, items)
	if len(plan.Remove) != 1 || plan.Remove[0].Dir != "notes-old" {
		t.Fatalf("removed %+v, want the older transcribed audio past the single slot", plan.Remove)
	}
	if plan.Protected != 0 {
		t.Fatalf("the newest file is inside the count, so it is kept rather than spared: protected %d", plan.Protected)
	}
}

func TestBuildPlanSkipsInUseAndOutside(t *testing.T) {
	now := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	// The live capture is old enough to expire, and still must not be deleted
	// out from under the recorder.
	live := audio("live", now.Add(-90*24*time.Hour), 10, true)
	live.InUse = true
	gone := audio("outside", now.Add(-90*24*time.Hour), 10, true)
	gone.Outside = true
	old := audio("old", now.Add(-91*24*time.Hour), 10, true)

	plan := BuildPlan(now, Policy{KeepDays: 1, KeepRecordings: 1}, []Item{live, gone, old})
	if len(plan.Remove) != 1 || plan.Remove[0].Dir != "old" {
		t.Fatalf("removed %+v, want only old", plan.Remove)
	}

	usage := Summarize([]Item{live, gone, old})
	if usage.Files != 3 || usage.Bytes != 30 {
		t.Fatalf("usage = %+v, outside audio still occupies disk", usage)
	}
}

func TestBuildPlanDisabledStillPreviews(t *testing.T) {
	now := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	items := []Item{audio("old", now.Add(-10*24*time.Hour), 2048, true)}
	p := Policy{Enabled: false, KeepDays: 1}
	plan := BuildPlan(now, p, items)
	if len(plan.Remove) != 1 {
		t.Fatal("preview must show the deletion even while auto-delete is off")
	}
	if p.Active() {
		t.Fatal("a disabled policy must not be active")
	}
	if got := PreviewLine(p, plan); got != "off — would free 2.0 KB across 1 file" {
		t.Fatalf("preview %q", got)
	}
}

func TestPreviewLineNoLimitAndProtection(t *testing.T) {
	if got := PreviewLine(Policy{}, Plan{}); got != "off — set keep_days or keep_recordings" {
		t.Fatalf("no limit: %q", got)
	}
	if got := PreviewLine(Policy{Enabled: true}, Plan{}); got != "no limit set — nothing is deleted" {
		t.Fatalf("enabled without caps: %q", got)
	}

	plan := Plan{Protected: 1, ProtectedBytes: 1024}
	got := PreviewLine(Policy{Enabled: true, KeepDays: 7, ProtectUntranscribed: true}, plan)
	if got != "nothing to delete — keeping 1 untranscribed recording (1.0 KB)" {
		t.Fatalf("protected only: %q", got)
	}
}

func TestApplyRemovesWavAndLeavesTranscript(t *testing.T) {
	dir := t.TempDir()
	wav := filepath.Join(dir, recorderAudio)
	if err := os.WriteFile(wav, []byte("audio-bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	transcript := filepath.Join(dir, "transcript.txt")
	if err := os.WriteFile(transcript, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	meta := filepath.Join(dir, "metadata.json")
	if err := os.WriteFile(meta, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}

	n, freed, err := Apply(Plan{Remove: []Removal{{
		Dir: dir, Path: wav, Bytes: int64(len("audio-bytes")), Reason: "older than 1 days",
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 || freed != int64(len("audio-bytes")) {
		t.Fatalf("removed %d freed %d", n, freed)
	}
	if _, err := os.Stat(wav); !os.IsNotExist(err) {
		t.Fatalf("wav still present: %v", err)
	}
	for _, keep := range []string{transcript, meta} {
		if _, err := os.Stat(keep); err != nil {
			t.Fatalf("kept file %s: %v", keep, err)
		}
	}
}

func TestApplyRefusesOtherNamesAndSymlinks(t *testing.T) {
	dir := t.TempDir()
	notes := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(notes, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Apply(Plan{Remove: []Removal{{Path: notes}}}); err == nil {
		t.Fatal("a non-audio path must be refused")
	}
	if _, err := os.Stat(notes); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(dir, "real.wav")
	if err := os.WriteFile(target, []byte("body"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, recorderAudio)
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Apply(Plan{Remove: []Removal{{Path: link}}}); err == nil {
		t.Fatal("a symlink must not be removed")
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("symlink target was removed: %v", err)
	}
}

func TestInsideLibrary(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "2026-09-01_meet")
	if !InsideLibrary(root, child) {
		t.Fatal("session dir under the output root must be inside the library")
	}
	if InsideLibrary(root, root) {
		t.Fatal("the output root itself is not a session")
	}
	if InsideLibrary(root, filepath.Dir(root)) {
		t.Fatal("a parent of the output root must be outside")
	}
	if InsideLibrary("", child) {
		t.Fatal("an unresolved output dir must not make every path deletable")
	}
}

func TestNegativeLimitsAreNotActive(t *testing.T) {
	p := Policy{Enabled: true, KeepDays: -5, KeepRecordings: -1}
	if p.Active() || p.LimitsSet() {
		t.Fatal("negative caps must behave as no limit, not as delete everything")
	}
	now := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	plan := BuildPlan(now, p, []Item{audio("old", now.Add(-48*time.Hour), 10, true)})
	if len(plan.Remove) != 0 {
		t.Fatal("negative caps must not select files")
	}
}

func audio(dir string, ended time.Time, size int64, transcript bool) Item {
	return Item{
		Dir:           dir,
		StartedAt:     ended.Add(-time.Hour),
		EndedAt:       ended,
		HasTranscript: transcript,
		HasAudio:      true,
		AudioBytes:    size,
	}
}

func paths(plan Plan) []string {
	out := make([]string, len(plan.Remove))
	for i, r := range plan.Remove {
		out[i] = r.Path
	}
	return out
}

func contains(paths []string, want string) bool {
	for _, p := range paths {
		if p == want {
			return true
		}
	}
	return false
}

// recorderAudio matches recorder.SessionAudioFile. Asserted so a rename of the
// capture file cannot leave retention deleting a name the recorder no longer writes.
const recorderAudio = "recording.wav"

func TestAudioPathUsesRecorderName(t *testing.T) {
	if filepath.Base(AudioPath("/tmp/sess")) != recorderAudio {
		t.Fatal(AudioPath("/tmp/sess"))
	}
}
