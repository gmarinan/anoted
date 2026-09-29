// Package retention decides which recording.wav files can be removed.
//
// Transcripts, metadata and the session row stay. The wav is what fills the
// disk; the notes are what the user still looks up afterwards.
package retention

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"anoted/internal/recorder"
)

// Policy is the user's audio retention rule.
type Policy struct {
	// Enabled is the opt-in. An existing library is never pruned until the
	// user turns automatic deletion on.
	Enabled bool
	// KeepDays removes audio whose meeting ended at least this many 24h
	// periods ago. Zero means no age limit.
	KeepDays int
	// KeepRecordings keeps this many newest audio files. Zero means no count
	// limit. A recording in progress occupies a slot so it is not pushed out
	// by its own capture.
	KeepRecordings int
	// ProtectUntranscribed refuses to delete audio that has no transcript yet.
	// Until Whisper has run, the wav is the only copy of that meeting.
	ProtectUntranscribed bool
}

// LimitsSet reports whether either cap would actually delete something.
func (p Policy) LimitsSet() bool {
	return p.normalized().KeepDays > 0 || p.normalized().KeepRecordings > 0
}

// Active reports whether a sweep should delete files. Preview still runs when
// this is false, so the config screen can show what turning it on would free.
func (p Policy) Active() bool {
	return p.Enabled && p.LimitsSet()
}

func (p Policy) normalized() Policy {
	if p.KeepDays < 0 {
		p.KeepDays = 0
	}
	if p.KeepRecordings < 0 {
		p.KeepRecordings = 0
	}
	return p
}

// Item is one session the sweep can see. Outside sessions still count toward
// disk usage, but they are not a license to unlink an arbitrary recording.wav
// a bad database row might point at.
type Item struct {
	Dir           string
	StartedAt     time.Time
	EndedAt       time.Time
	HasTranscript bool
	HasAudio      bool
	AudioBytes    int64
	// InUse is the recording being captured or transcribed right now.
	InUse bool
	// Outside is a session directory that does not live under the configured
	// output directory.
	Outside bool
}

// Removal is one wav the plan wants deleted.
type Removal struct {
	Dir    string
	Path   string
	Bytes  int64
	Reason string
}

// Plan is what a sweep would do. It does not touch the disk.
type Plan struct {
	Remove         []Removal
	Protected      int
	ProtectedBytes int64
}

// Usage is how much recording audio is on disk.
type Usage struct {
	Files              int
	Bytes              int64
	UntranscribedFiles int
	UntranscribedBytes int64
}

// AudioPath is the mixed recording inside a session directory.
func AudioPath(dir string) string {
	return filepath.Join(dir, recorder.SessionAudioFile)
}

// InsideLibrary reports whether dir is a child of the configured output root.
// The root itself is not a session directory.
func InsideLibrary(root, dir string) bool {
	if root == "" || dir == "" {
		return false
	}
	root = filepath.Clean(root)
	dir = filepath.Clean(dir)
	rel, err := filepath.Rel(root, dir)
	if err != nil {
		return false
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return false
	}
	return !filepath.IsAbs(rel)
}

// Summarize totals audio bytes. Sessions without a wav contribute nothing.
func Summarize(items []Item) Usage {
	var u Usage
	for _, it := range items {
		if !it.HasAudio {
			continue
		}
		u.Files++
		u.Bytes += it.AudioBytes
		if !it.HasTranscript {
			u.UntranscribedFiles++
			u.UntranscribedBytes += it.AudioBytes
		}
	}
	return u
}

func (u Usage) String() string {
	if u.Files == 0 {
		return "0 B"
	}
	s := fmt.Sprintf("%s · %s", FormatBytes(u.Bytes), quantity(u.Files, "file", "files"))
	if u.UntranscribedFiles > 0 {
		s += fmt.Sprintf(" · %s not transcribed", FormatBytes(u.UntranscribedBytes))
	}
	return s
}

// BuildPlan chooses wavs to delete. Enabled is ignored so the config screen
// can preview a policy that is still off. In-use and outside sessions are
// never selected. Untranscribed audio is skipped when protection is on, and
// those spared files are counted on the plan.
func BuildPlan(now time.Time, p Policy, items []Item) Plan {
	p = p.normalized()
	if !p.LimitsSet() {
		return Plan{}
	}

	ranked := make([]Item, 0, len(items))
	for _, it := range items {
		if !it.HasAudio || it.Dir == "" || it.Outside {
			continue
		}
		ranked = append(ranked, it)
	}
	sort.Slice(ranked, func(i, j int) bool {
		ti, tj := effectiveTime(ranked[i]), effectiveTime(ranked[j])
		if !ti.Equal(tj) {
			return ti.After(tj)
		}
		return ranked[i].Dir < ranked[j].Dir
	})

	var cutoff time.Time
	if p.KeepDays > 0 {
		cutoff = now.Add(-time.Duration(p.KeepDays) * 24 * time.Hour)
	}

	var plan Plan
	for i, it := range ranked {
		overCount := p.KeepRecordings > 0 && i >= p.KeepRecordings
		ended := effectiveTime(it)
		overAge := p.KeepDays > 0 && !ended.IsZero() && !ended.After(cutoff)
		if !overCount && !overAge {
			continue
		}
		if it.InUse {
			continue
		}
		if p.ProtectUntranscribed && !it.HasTranscript {
			plan.Protected++
			plan.ProtectedBytes += it.AudioBytes
			continue
		}
		plan.Remove = append(plan.Remove, Removal{
			Dir:    it.Dir,
			Path:   AudioPath(it.Dir),
			Bytes:  it.AudioBytes,
			Reason: removalReason(overAge, overCount, p.KeepDays, p.KeepRecordings),
		})
	}
	return plan
}

func effectiveTime(it Item) time.Time {
	if !it.EndedAt.IsZero() {
		return it.EndedAt
	}
	return it.StartedAt
}

func removalReason(overAge, overCount bool, days, count int) string {
	switch {
	case overAge && overCount:
		return fmt.Sprintf("older than %d days and past the newest %d", days, count)
	case overAge:
		return fmt.Sprintf("older than %d days", days)
	default:
		return fmt.Sprintf("past the newest %d", count)
	}
}

// PreviewLine describes the plan in the config screen. A disabled policy still
// says what enabling it would free, so the user can see the effect first.
func PreviewLine(p Policy, plan Plan) string {
	p = p.normalized()
	if !p.LimitsSet() {
		if p.Enabled {
			return "no limit set — nothing is deleted"
		}
		return "off — set keep_days or keep_recordings"
	}

	freed, n := plan.totals()
	keep := keepingClause(plan)
	switch {
	case n == 0 && keep == "":
		if p.Enabled {
			return "nothing to delete"
		}
		return "off — nothing would be deleted"
	case n == 0:
		if p.Enabled {
			return "nothing to delete — " + keep
		}
		return "off — " + keep
	case keep == "":
		action := fmt.Sprintf("%s across %s", FormatBytes(freed), quantity(n, "file", "files"))
		if p.Enabled {
			return "would free " + action
		}
		return "off — would free " + action
	default:
		action := fmt.Sprintf("%s across %s — %s", FormatBytes(freed), quantity(n, "file", "files"), keep)
		if p.Enabled {
			return "would free " + action
		}
		return "off — would free " + action
	}
}

func (p Plan) totals() (int64, int) {
	var bytes int64
	for _, r := range p.Remove {
		bytes += r.Bytes
	}
	return bytes, len(p.Remove)
}

func keepingClause(plan Plan) string {
	if plan.Protected == 0 {
		return ""
	}
	noun := "recordings"
	if plan.Protected == 1 {
		noun = "recording"
	}
	return fmt.Sprintf("keeping %d untranscribed %s (%s)", plan.Protected, noun, FormatBytes(plan.ProtectedBytes))
}

// Apply deletes the planned wavs and nothing else. A missing file was already
// removed; a path that is not recording.wav is refused.
func Apply(plan Plan) (removed int, freed int64, err error) {
	var errs []error
	for _, r := range plan.Remove {
		if rmErr := removeAudio(r.Path); rmErr != nil {
			if errors.Is(rmErr, os.ErrNotExist) {
				continue
			}
			errs = append(errs, rmErr)
			continue
		}
		removed++
		freed += r.Bytes
		slog.Info("audio retention removed recording", "path", r.Path, "bytes", r.Bytes, "reason", r.Reason)
	}
	return removed, freed, errors.Join(errs...)
}

func removeAudio(path string) error {
	if filepath.Base(path) != recorder.SessionAudioFile {
		return fmt.Errorf("refusing to delete %s", path)
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("refusing to delete non-regular file %s", path)
	}
	return os.Remove(path)
}

// FormatBytes renders a size the way the rest of the UI talks about disk.
func FormatBytes(n int64) string {
	if n < 0 {
		n = 0
	}
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
	)
	switch {
	case n < kb:
		return fmt.Sprintf("%d B", n)
	case n < mb:
		return fmt.Sprintf("%.1f KB", float64(n)/kb)
	case n < gb:
		return fmt.Sprintf("%.1f MB", float64(n)/mb)
	default:
		return fmt.Sprintf("%.1f GB", float64(n)/float64(gb))
	}
}

func quantity(n int, singular, plural string) string {
	if n == 1 {
		return "1 " + singular
	}
	return fmt.Sprintf("%d %s", n, plural)
}
