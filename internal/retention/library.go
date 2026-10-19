package retention

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"

	"anoted/internal/config"
	"anoted/internal/session"
	"anoted/internal/transcribe"
)

// PolicyFrom turns the retention section of the config into a Policy.
func PolicyFrom(r config.RetentionConfig) Policy {
	return Policy{
		Enabled:              r.AutoDelete,
		KeepDays:             r.KeepDays,
		KeepRecordings:       r.KeepRecordings,
		ProtectUntranscribed: r.ProtectsUntranscribed(),
	}
}

// Scan describes the audio of each session for BuildPlan. inUse holds the
// dirs being captured or transcribed right now. A row still marked active is
// treated the same way: either it is recording, or a crash left it behind and
// nobody has looked at it yet.
func Scan(root string, recs []session.Record, tcfg config.TranscriptionConfig, inUse map[string]struct{}) []Item {
	items := make([]Item, 0, len(recs))
	seen := make(map[string]struct{}, len(recs))
	for _, r := range recs {
		if r.Dir == "" {
			continue
		}
		if _, dup := seen[r.Dir]; dup {
			continue
		}
		seen[r.Dir] = struct{}{}

		files := transcribe.AllTranscriptFiles(r.Dir, tcfg)
		it := Item{
			Dir:           r.Dir,
			StartedAt:     r.StartedAt,
			EndedAt:       r.EndedAt,
			HasTranscript: len(files) > 0,
		}
		info, err := os.Lstat(AudioPath(r.Dir))
		if err == nil && info.Mode().IsRegular() {
			it.HasAudio = true
			it.AudioBytes = info.Size()
		}
		// Only audio that could be deleted is worth opening transcripts for.
		if it.HasAudio && it.HasTranscript {
			it.EmptyTranscript = !anyTranscriptText(files)
		}
		if _, live := inUse[r.Dir]; live || r.Status == session.StatusActive {
			it.InUse = true
		}
		if !InsideLibrary(root, r.Dir) {
			it.Outside = true
		}
		items = append(items, it)
	}
	return items
}

// transcriptPeek bounds how much of a transcript is read to find words. Any
// real transcript has them in its first few lines.
const transcriptPeek = 64 << 10

func anyTranscriptText(paths []string) bool {
	for _, p := range paths {
		if transcriptHasText(p) {
			return true
		}
	}
	return false
}

// transcriptHasText reports whether a transcript file holds any words.
//
// Whisper writes a complete set of files for a silent or unreadable wav: an
// empty txt and srt, a bare WEBVTT header, a JSON object with no text, and a
// markdown note that is only frontmatter. Counting those as transcribed would
// delete the one copy of a meeting whose transcription went wrong.
func transcriptHasText(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	buf, err := io.ReadAll(io.LimitReader(f, transcriptPeek+1))
	if err != nil {
		return false
	}
	truncated := len(buf) > transcriptPeek

	switch strings.ToLower(filepath.Ext(path)) {
	case ".vtt":
		buf = bytes.TrimPrefix(bytes.TrimSpace(buf), []byte("WEBVTT"))
	case ".json":
		if truncated {
			// Too large to be an empty result.
			return true
		}
		var doc struct {
			Text     string `json:"text"`
			Segments []struct {
				Text string `json:"text"`
			} `json:"segments"`
		}
		if json.Unmarshal(buf, &doc) != nil {
			return false
		}
		if strings.TrimSpace(doc.Text) != "" {
			return true
		}
		for _, s := range doc.Segments {
			if strings.TrimSpace(s.Text) != "" {
				return true
			}
		}
		return false
	case ".md":
		body, ok := markdownBody(buf)
		if !ok {
			// Frontmatter that never closes within the peek is not anoted's.
			return truncated
		}
		buf = body
	}
	return len(bytes.TrimSpace(buf)) > 0
}

// markdownBody strips the YAML frontmatter WriteMeetingMarkdown puts on top.
func markdownBody(b []byte) ([]byte, bool) {
	if !bytes.HasPrefix(b, []byte("---\n")) {
		return b, true
	}
	rest := b[len("---\n"):]
	end := bytes.Index(rest, []byte("\n---\n"))
	if end < 0 {
		return nil, false
	}
	return rest[end+len("\n---\n"):], true
}

// Without drops removals for dirs that became busy after the plan was built.
func (p Plan) Without(busy map[string]struct{}) Plan {
	if len(busy) == 0 {
		return p
	}
	kept := make([]Removal, 0, len(p.Remove))
	for _, r := range p.Remove {
		if _, skip := busy[r.Dir]; skip {
			continue
		}
		kept = append(kept, r)
	}
	p.Remove = kept
	return p
}
