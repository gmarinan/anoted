package tui

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"anoted/internal/config"
	"anoted/internal/retention"
	"anoted/internal/session"
	"anoted/internal/transcribe"
	"anoted/internal/tui/components"
	tea "charm.land/bubbletea/v2"
)

func retentionPolicy(r config.RetentionConfig) retention.Policy {
	return retention.Policy{
		Enabled:              r.AutoDelete,
		KeepDays:             r.KeepDays,
		KeepRecordings:       r.KeepRecordings,
		ProtectUntranscribed: r.ProtectsUntranscribed(),
	}
}

func (m Model) retentionPolicy() retention.Policy {
	return retentionPolicy(m.deps.Config.Retention)
}

// audioInUse is the set of session dirs whose wav is open right now.
//
// Deleting the file under an active capture or an active Whisper run would
// truncate the recording or fail the transcription halfway through.
func (m Model) audioInUse() map[string]struct{} {
	inUse := make(map[string]struct{})
	if m.recording && m.sessionDir != "" {
		inUse[m.sessionDir] = struct{}{}
	}
	if m.transcribeActive && m.transcribeSessionDir != "" {
		inUse[m.transcribeSessionDir] = struct{}{}
	}
	for _, dir := range m.transcribeQueue {
		if dir != "" {
			inUse[dir] = struct{}{}
		}
	}
	return inUse
}

type librarySnap struct {
	usage   retention.Usage
	preview string
	plan    retention.Plan
	policy  retention.Policy
	items   []retention.Item
	scanned bool
}

func planLibrary(recs []session.Record, cfg config.Config, inUse map[string]struct{}) librarySnap {
	pol := retentionPolicy(cfg.Retention)
	root, err := cfg.ResolvedOutputDir()
	if err != nil {
		// An unresolved root must not make every session dir look deletable.
		root = ""
	}
	items := libraryItems(root, recs, cfg.Transcription, inUse)
	plan := retention.BuildPlan(time.Now(), pol, items)
	return librarySnap{
		usage:   retention.Summarize(items),
		preview: retention.PreviewLine(pol, plan),
		plan:    plan,
		policy:  pol,
		items:   items,
		scanned: true,
	}
}

func libraryItems(root string, recs []session.Record, tcfg config.TranscriptionConfig, inUse map[string]struct{}) []retention.Item {
	items := make([]retention.Item, 0, len(recs))
	seen := make(map[string]struct{}, len(recs))
	for _, r := range recs {
		if r.Dir == "" {
			continue
		}
		if _, dup := seen[r.Dir]; dup {
			continue
		}
		seen[r.Dir] = struct{}{}

		it := retention.Item{
			Dir:           r.Dir,
			StartedAt:     r.StartedAt,
			EndedAt:       r.EndedAt,
			HasTranscript: transcribe.HasTranscript(r.Dir, tcfg),
		}
		info, err := os.Lstat(retention.AudioPath(r.Dir))
		if err == nil && info.Mode().IsRegular() {
			it.HasAudio = true
			it.AudioBytes = info.Size()
		}
		if _, live := inUse[r.Dir]; live {
			it.InUse = true
		}
		if !retention.InsideLibrary(root, r.Dir) {
			it.Outside = true
		}
		items = append(items, it)
	}
	return items
}

func artifactsFromItems(recs []session.Record, items []retention.Item) map[string]components.SessionArtifacts {
	byDir := make(map[string]retention.Item, len(items))
	for _, it := range items {
		byDir[it.Dir] = it
	}
	facts := make(map[string]components.SessionArtifacts, len(recs))
	for _, r := range recs {
		if r.Dir == "" {
			continue
		}
		it, ok := byDir[r.Dir]
		if !ok {
			continue
		}
		facts[r.Dir] = components.SessionArtifacts{
			HasTranscript: it.HasTranscript,
			HasAudio:      it.HasAudio,
			AudioBytes:    it.AudioBytes,
		}
	}
	return facts
}

type audioLibraryMsg struct {
	usage   retention.Usage
	preview string
	plan    retention.Plan
	policy  retention.Policy
	scanned bool
}

func (m Model) refreshAudioLibraryCmd() tea.Cmd {
	store := m.deps.Store
	cfg := m.deps.Config
	inUse := m.audioInUse()
	return func() tea.Msg {
		recs, err := loadSessionRecords(store)
		if err != nil {
			slog.Warn("audio library scan failed", "err", err)
			return audioLibraryMsg{preview: "could not measure audio"}
		}
		snap := planLibrary(recs, cfg, inUse)
		return audioLibraryMsg{
			usage:   snap.usage,
			preview: snap.preview,
			plan:    snap.plan,
			policy:  snap.policy,
			scanned: snap.scanned,
		}
	}
}

type retentionDoneMsg struct {
	removed   int
	freed     int64
	protected int
	err       error
}

// retentionFollowUp deletes only when the plan was built for the policy that
// is still in effect. A scan that started before the user turned auto-delete
// off must not unlink files on the way out.
func (m Model) retentionFollowUp(pol retention.Policy, plan retention.Plan) tea.Cmd {
	if m.retentionPolicy() != pol || !pol.Active() || len(plan.Remove) == 0 {
		return nil
	}
	protected := plan.Protected
	return func() tea.Msg {
		n, freed, err := retention.Apply(plan)
		return retentionDoneMsg{removed: n, freed: freed, protected: protected, err: err}
	}
}

func (m Model) handleAudioLibrary(msg audioLibraryMsg) (tea.Model, tea.Cmd) {
	m = m.storeAudioScan(msg.scanned, msg.usage, msg.preview)
	return m, m.retentionFollowUp(msg.policy, msg.plan)
}

func (m Model) handleRetentionDone(msg retentionDoneMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		slog.Warn("audio retention failed", "err", msg.err, "removed", msg.removed)
	}
	if msg.removed == 0 {
		return m, nil
	}
	note := fmt.Sprintf("Freed %s (%s). Transcripts kept.", retention.FormatBytes(msg.freed), recordingsNoun(msg.removed))
	if msg.protected > 0 {
		note += fmt.Sprintf(" %s left in place.", untranscribedNoun(msg.protected))
	}
	m.statusNote = note
	m.markStatusTransient()
	return m, m.loadSessionsCmd()
}

func recordingsNoun(n int) string {
	if n == 1 {
		return "1 recording"
	}
	return fmt.Sprintf("%d recordings", n)
}

func untranscribedNoun(n int) string {
	if n == 1 {
		return "1 untranscribed recording"
	}
	return fmt.Sprintf("%d untranscribed recordings", n)
}

func (m Model) storeAudioScan(scanned bool, usage retention.Usage, preview string) Model {
	if scanned {
		m.audioScanned = true
		m.audioUsage = usage
	}
	if preview != "" {
		m.audioPreview = preview
	}
	return m
}

func (m Model) audioUsageText() string {
	if !m.audioScanned {
		return ""
	}
	if m.audioUsage.Files == 0 {
		if len(m.sessions) > 0 {
			return "0 B — no audio on disk"
		}
		return "0 B"
	}
	return m.audioUsage.String()
}
