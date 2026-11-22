package tui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"anoted/internal/config"
	"anoted/internal/filejob"
	"anoted/internal/transcribe"
	tea "charm.land/bubbletea/v2"
)

func filesModel(t *testing.T) Model {
	t.Helper()
	return Model{
		screen: ScreenFiles,
		width:  110,
		height: 40,
		scroll: newSessionScroll(),
		cache:  &viewCache{},
		deps: Deps{
			Transcriber: transcribe.New(config.Config{}),
			FilesRoot:   filepath.Join(t.TempDir(), "files"),
			Opener:      systemOpener{},
		},
	}
}

func mediaFile(t *testing.T, name string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte("not really audio"), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// run executes a command and feeds its message back, as the runtime would.
func run(t *testing.T, m Model, cmd tea.Cmd) Model {
	t.Helper()
	if cmd == nil {
		return m
	}
	msg := cmd()
	if msg == nil {
		return m
	}
	next, _ := m.Update(msg)
	return next.(Model)
}

func TestDroppedFileIsQueuedBehindAMeeting(t *testing.T) {
	m := filesModel(t)
	// A meeting is already transcribing: the file must wait for it, not start
	// a second engine on the GPU.
	m.transcribeActive = true
	m.transcribeSessionDir = "/meetings/1"

	src := mediaFile(t, "voice note.m4a")
	next, cmd := m.Update(tea.PasteMsg{Content: src + " "})
	m = run(t, next.(Model), cmd)

	if len(m.fileJobs) != 1 || m.fileJobs[0].Source != src {
		t.Fatalf("jobs = %+v", m.fileJobs)
	}
	if m.transcribeSessionDir != "/meetings/1" {
		t.Fatal("the meeting lost its turn")
	}
	if len(m.transcribeQueue) != 1 || m.transcribeQueue[0] != m.fileJobs[0].Dir {
		t.Fatalf("queue = %v", m.transcribeQueue)
	}
	if state, pos := m.fileState(m.fileJobs[0]); pos != 1 || state == 0 {
		t.Fatalf("state = %v pos = %d", state, pos)
	}
}

func TestPasteOnHomeOnlyActsOnMediaFiles(t *testing.T) {
	m := filesModel(t)
	m.screen = ScreenMain

	// Stray clipboard text on Home: nothing happens, nothing is said.
	next, cmd := m.Update(tea.PasteMsg{Content: "some text I copied"})
	m = run(t, next.(Model), cmd)
	if len(m.fileJobs) != 0 || m.fileNote != "" || m.screen != ScreenMain {
		t.Fatalf("stray paste acted on: jobs=%d note=%q screen=%s", len(m.fileJobs), m.fileNote, m.screen)
	}

	// A dropped audio file: switch to Files and add it.
	m.transcribeActive = true // keep the job queued instead of starting an engine
	next, cmd = m.Update(tea.PasteMsg{Content: "'" + mediaFile(t, "call.mp3") + "'"})
	m = run(t, next.(Model), cmd)
	if m.screen != ScreenFiles || len(m.fileJobs) != 1 {
		t.Fatalf("drop on Home: screen=%s jobs=%d", m.screen, len(m.fileJobs))
	}
}

func TestPasteInConfigIsIgnored(t *testing.T) {
	m := filesModel(t)
	m.screen = ScreenConfig
	_, cmd := m.Update(tea.PasteMsg{Content: mediaFile(t, "a.wav")})
	if cmd != nil {
		t.Fatal("a paste on Config was treated as a dropped file")
	}
}

func TestTypedPathKeepsQ(t *testing.T) {
	m := filesModel(t)
	next, _ := m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	m = next.(Model)
	for _, r := range "/tmp/q" {
		next, cmd := m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		if cmd != nil {
			t.Fatalf("typing %q produced a command (quit?)", r)
		}
		m = next.(Model)
	}
	if !m.fileEditing || m.fileInput != "/tmp/q" {
		t.Fatalf("editing=%v input=%q", m.fileEditing, m.fileInput)
	}
	next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if next.(Model).fileEditing {
		t.Fatal("esc did not cancel")
	}
}

func TestMissingPathIsReported(t *testing.T) {
	m := filesModel(t)
	next, cmd := m.Update(tea.PasteMsg{Content: "/no/such/file.mp3"})
	m = run(t, next.(Model), cmd)
	if len(m.fileJobs) != 0 || !m.fileNoteErr || !strings.Contains(m.fileNote, "no such file") {
		t.Fatalf("note=%q err=%v", m.fileNote, m.fileNoteErr)
	}
}

func TestFailedFileDoesNotReplaceTheSessionsTable(t *testing.T) {
	m := filesModel(t)
	job, err := filejob.Create(m.deps.FilesRoot, mediaFile(t, "x.ogg"), "", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	m.fileJobs = []filejob.Job{job}
	m.transcribeActive = true
	m.transcribeSessionDir = job.Dir

	failure := errors.New("parakeet: boom")
	next, _ := m.handleTranscribeResult(transcribeResultMsg{
		err:        failure,
		sessionDir: job.Dir,
		file:       finishFileJob(job, transcribe.Result{}, failure),
	})
	m = next.(Model)
	if m.sessionsErr != "" || m.transcribeErr != "" {
		t.Fatalf("a file failure leaked into Home: sessionsErr=%q transcribeErr=%q", m.sessionsErr, m.transcribeErr)
	}
	if m.fileJobs[0].Error == "" {
		t.Fatal("the failure was not recorded on the job")
	}
	// It survives a reload from disk.
	jobs, _ := filejob.List(m.deps.FilesRoot)
	if len(jobs) != 1 || jobs[0].Error != "parakeet: boom" {
		t.Fatalf("on disk: %+v", jobs)
	}
}

func TestFinishedFileShowsAndCopiesItsTranscript(t *testing.T) {
	m := filesModel(t)
	job, err := filejob.Create(m.deps.FilesRoot, mediaFile(t, "x.ogg"), "es", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(job.TranscriptPath(), []byte("hola qué tal\nbien\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	m.fileJobs = []filejob.Job{job}
	m.transcribeActive = true
	m.transcribeSessionDir = job.Dir

	next, _ := m.handleTranscribeResult(transcribeResultMsg{
		sessionDir: job.Dir,
		file:       finishFileJob(job, transcribe.Result{AudioDuration: 90 * time.Second}, nil),
	})
	m = next.(Model)
	if m.fileText != "hola qué tal\nbien" || m.fileWords != 4 || len(m.fileLines) != 2 {
		t.Fatalf("text=%q words=%d lines=%v", m.fileText, m.fileWords, m.fileLines)
	}
	if m.fileJobs[0].Duration() != 90*time.Second {
		t.Fatalf("duration %v", m.fileJobs[0].Duration())
	}

	next, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	m = next.(Model)
	if cmd == nil || !strings.Contains(m.fileNote, "copied 4 words") {
		t.Fatalf("copy: cmd=%v note=%q", cmd != nil, m.fileNote)
	}
}

func TestRemoveAsksTwiceAndLeavesTheQueue(t *testing.T) {
	m := filesModel(t)
	job, err := filejob.Create(m.deps.FilesRoot, mediaFile(t, "x.ogg"), "", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	m.fileJobs = []filejob.Job{job}
	m.transcribeActive = true
	m.transcribeSessionDir = "/meetings/1"
	m.transcribeQueue = []string{job.Dir, "/meetings/2"}

	next, cmd := m.Update(tea.KeyPressMsg{Code: 'd', Text: "d"})
	m = next.(Model)
	if cmd != nil || len(m.fileJobs) != 1 {
		t.Fatal("removed on the first press")
	}
	next, cmd = m.Update(tea.KeyPressMsg{Code: 'd', Text: "d"})
	m = run(t, next.(Model), cmd)
	if len(m.fileJobs) != 0 {
		t.Fatalf("jobs = %v", m.fileJobs)
	}
	if strings.Join(m.transcribeQueue, ",") != "/meetings/2" {
		t.Fatalf("queue = %v", m.transcribeQueue)
	}
	if _, err := os.Stat(job.Dir); !os.IsNotExist(err) {
		t.Fatal("job folder still on disk")
	}
	if _, err := os.Stat(job.Source); err != nil {
		t.Fatal("the source file was deleted")
	}
}

func TestLanguageCyclesFromTheMeetingLanguage(t *testing.T) {
	m := filesModel(t)
	m.deps.Config.Transcription.Language = "es"
	if m.fileLanguage() != "es" {
		t.Fatal("does not start from the meeting language")
	}
	seen := []string{}
	// "es" is already among the choices, so the cycle is exactly that long.
	for i := 0; i < len(fileLanguageChoices); i++ {
		next, _ := m.Update(tea.KeyPressMsg{Code: 'l', Text: "l"})
		m = next.(Model)
		seen = append(seen, m.fileLanguage())
	}
	if seen[0] != "" || seen[len(seen)-1] != "es" {
		t.Fatalf("cycle = %q", seen)
	}
}

func TestFilesViewIsPure(t *testing.T) {
	m := filesModel(t)
	m.fileJobs = []filejob.Job{{Dir: "/x/1", Name: "a.mp3", Done: true}, {Dir: "/x/2", Name: "b.mp3"}}
	m = m.setFileText("/x/1", strings.Repeat("palabra ", 400))
	first := m.View()
	second := m.View()
	if first.Content != second.Content {
		t.Fatal("Files view is not a pure function of the Model")
	}
	if n := strings.Count(first.Content, "\n") + 1; n != m.height {
		t.Fatalf("frame is %d rows, terminal is %d", n, m.height)
	}
	if strings.Contains(first.Content, "more rows") {
		t.Fatal("the Files screen overflows a 40-row terminal")
	}
}

func TestFileScrollClamps(t *testing.T) {
	m := filesModel(t)
	m.fileJobs = []filejob.Job{{Dir: "/x/1", Name: "a.mp3", Done: true}}
	m = m.setFileText("/x/1", strings.Repeat("línea\n", 200))
	m = m.scrollFileText(-5)
	if m.fileScroll != 0 {
		t.Fatalf("scroll %d", m.fileScroll)
	}
	m = m.scrollFileText(10000)
	if want := len(m.fileLines) - m.fileTextRows(); m.fileScroll != want {
		t.Fatalf("scroll %d, want %d", m.fileScroll, want)
	}
}
