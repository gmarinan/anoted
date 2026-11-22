// Package filejob keeps the standalone audio files a user hands to anoted for
// transcription: one folder per file, holding a small job.json and the
// transcript. The source file itself is never copied, moved or deleted.
//
// These live apart from the meeting library on purpose. The library root is
// swept by reconciliation and by the audio retention policy, both of which
// treat any folder there as a recording session.
package filejob

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"anoted/internal/session"
	"anoted/internal/transcribe"
)

const (
	metaFile = "job.json"
	// TranscriptFile is the transcript's name inside a job folder.
	TranscriptFile = transcribe.TranscriptBaseName + ".txt"
)

// Job is one file handed over for transcription.
type Job struct {
	// Dir is the job folder. It is the job's identity everywhere, including in
	// the TUI's transcription queue, and is derived from the path on load.
	Dir string `json:"-"`
	// Done reports whether a transcript is on disk.
	Done bool `json:"-"`

	Source  string    `json:"source"`
	Name    string    `json:"name"`
	AddedAt time.Time `json:"added_at"`
	// Language is the code the job runs with; empty lets the engine detect it.
	Language    string  `json:"language,omitempty"`
	DurationSec float64 `json:"duration_sec,omitempty"`
	// Error is the last run's failure, cleared when a run succeeds.
	Error string `json:"error,omitempty"`
}

// Duration is the audio length, or zero before the first run has measured it.
func (j Job) Duration() time.Duration {
	return time.Duration(j.DurationSec * float64(time.Second))
}

// TranscriptPath is where the job's transcript is written.
func (j Job) TranscriptPath() string {
	return filepath.Join(j.Dir, TranscriptFile)
}

// Root is the folder holding every job, next to anoted's other managed data.
func Root() string {
	return filepath.Join(filepath.Dir(transcribe.ManagedVenvDir()), "files")
}

// Create registers source as a new job under root.
func Create(root, source, language string, now time.Time) (Job, error) {
	abs, err := filepath.Abs(source)
	if err != nil {
		return Job{}, fmt.Errorf("resolve %s: %w", source, err)
	}
	st, err := os.Stat(abs)
	if err != nil {
		return Job{}, fmt.Errorf("%s: no such file", abs)
	}
	if st.IsDir() {
		return Job{}, fmt.Errorf("%s is a folder", abs)
	}
	if err := os.MkdirAll(root, session.SessionDirMode); err != nil {
		return Job{}, fmt.Errorf("create %s: %w", root, err)
	}

	base := now.Format("20060102-150405") + "-" + slug(filepath.Base(abs))
	dir := filepath.Join(root, base)
	// Mkdir, not MkdirAll: two files dropped in the same second with the same
	// name must not share a folder.
	for n := 2; ; n++ {
		err := os.Mkdir(dir, session.SessionDirMode)
		if err == nil {
			break
		}
		if !errors.Is(err, os.ErrExist) || n > 100 {
			return Job{}, fmt.Errorf("create %s: %w", dir, err)
		}
		dir = filepath.Join(root, fmt.Sprintf("%s-%d", base, n))
	}

	job := Job{
		Dir:      dir,
		Source:   abs,
		Name:     filepath.Base(abs),
		AddedAt:  now,
		Language: language,
	}
	if err := job.Save(); err != nil {
		_ = os.RemoveAll(dir)
		return Job{}, err
	}
	return job, nil
}

// Save writes the job's metadata into its folder.
func (j Job) Save() error {
	if j.Dir == "" {
		return errors.New("job has no folder")
	}
	data, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		return fmt.Errorf("encode job: %w", err)
	}
	path := filepath.Join(j.Dir, metaFile)
	if err := os.WriteFile(path, append(data, '\n'), session.SessionFileMode); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// List returns every job under root, newest first. A missing root is an empty
// list, not an error: nothing has been added yet.
func List(root string) ([]Job, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", root, err)
	}
	var jobs []Job
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		job, err := load(filepath.Join(root, e.Name()))
		if err != nil {
			// A folder without readable metadata is not a job; skipping it
			// keeps one damaged entry from hiding all the others.
			continue
		}
		jobs = append(jobs, job)
	}
	sort.SliceStable(jobs, func(a, b int) bool {
		if jobs[a].AddedAt.Equal(jobs[b].AddedAt) {
			return jobs[a].Dir > jobs[b].Dir
		}
		return jobs[a].AddedAt.After(jobs[b].AddedAt)
	})
	return jobs, nil
}

func load(dir string) (Job, error) {
	data, err := os.ReadFile(filepath.Join(dir, metaFile))
	if err != nil {
		return Job{}, err
	}
	var job Job
	if err := json.Unmarshal(data, &job); err != nil {
		return Job{}, err
	}
	job.Dir = dir
	if job.Name == "" {
		job.Name = filepath.Base(job.Source)
	}
	if st, err := os.Stat(job.TranscriptPath()); err == nil && !st.IsDir() {
		job.Done = true
	}
	return job, nil
}

// ReadTranscript returns the job's transcript text.
func ReadTranscript(dir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(dir, TranscriptFile))
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(data), "\n"), nil
}

// Remove deletes a job folder — the transcript and metadata, never the source.
//
// It refuses anything that is not a job directly under root. The folder comes
// from in-memory state, and an empty or mangled value handed to RemoveAll is
// the kind of bug that deletes a home directory.
func Remove(root, dir string) error {
	if root == "" || dir == "" {
		return errors.New("refusing to remove: empty path")
	}
	root, dir = filepath.Clean(root), filepath.Clean(dir)
	if filepath.Dir(dir) != root {
		return fmt.Errorf("refusing to remove %s: not inside %s", dir, root)
	}
	if _, err := os.Stat(filepath.Join(dir, metaFile)); err != nil {
		return fmt.Errorf("refusing to remove %s: not a transcription job", dir)
	}
	return os.RemoveAll(dir)
}

// slug reduces a file name to something safe as a folder name on every
// platform, keeping it recognisable.
func slug(name string) string {
	name = strings.TrimSuffix(name, filepath.Ext(name))
	var b strings.Builder
	dash := false
	for _, r := range name {
		switch {
		case r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)):
			b.WriteRune(unicode.ToLower(r))
			dash = false
		case b.Len() > 0 && !dash:
			b.WriteByte('-')
			dash = true
		}
		if b.Len() >= 40 {
			break
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "audio"
	}
	return out
}
