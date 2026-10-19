package tui

import (
	"fmt"
	"log/slog"
	"time"

	"anoted/internal/config"
	"anoted/internal/retention"
	"anoted/internal/session"
	"anoted/internal/tui/components"
	tea "charm.land/bubbletea/v2"
)

func (m Model) retentionPolicy() retention.Policy {
	return retention.PolicyFrom(m.deps.Config.Retention)
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
	pol := retention.PolicyFrom(cfg.Retention)
	root, err := cfg.ResolvedOutputDir()
	if err != nil {
		// An unresolved root must not make every session dir look deletable.
		root = ""
	}
	items := retention.Scan(root, recs, cfg.Transcription, inUse)
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
// off must not unlink files on the way out. The same goes for a session that
// started recording or transcribing while the scan ran.
func (m Model) retentionFollowUp(pol retention.Policy, plan retention.Plan) tea.Cmd {
	if m.retentionPolicy() != pol || !pol.Active() {
		return nil
	}
	plan = plan.Without(m.audioInUse())
	if len(plan.Remove) == 0 {
		return nil
	}
	protected := plan.Protected
	return func() tea.Msg {
		n, freed, err := retention.Apply(plan)
		return retentionDoneMsg{removed: n, freed: freed, protected: protected, err: err}
	}
}

// retentionSweepInterval re-applies the policy while anoted sits idle.
// keep_days is a clock rule: without the sweep, audio only aged out when a
// recording or a transcription happened to reload the library on the main
// screen.
const retentionSweepInterval = time.Hour

type retentionSweepMsg struct{}

func scheduleRetentionSweep() tea.Cmd {
	return tea.Tick(retentionSweepInterval, func(time.Time) tea.Msg { return retentionSweepMsg{} })
}

func (m Model) handleRetentionSweep() (tea.Model, tea.Cmd) {
	return m, tea.Batch(scheduleRetentionSweep(), m.retentionSweepScan())
}

// retentionSweepScan rescans only when the scan could delete something; the
// Storage screen refreshes its own preview when it is opened or saved.
func (m Model) retentionSweepScan() tea.Cmd {
	if !m.retentionPolicy().Active() {
		return nil
	}
	return m.refreshAudioLibraryCmd()
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
