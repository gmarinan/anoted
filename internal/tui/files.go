package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"anoted/internal/filejob"
	"anoted/internal/open"
	"anoted/internal/tui/components"
	tea "charm.land/bubbletea/v2"
)

// The Files tab transcribes audio that did not come from a meeting: a voice
// note, a call recording, a video. A file is handed over by dragging it onto
// the terminal — which pastes its path — or by typing the path.
//
// A file job rides the same transcription queue as the meetings. That is not
// a convenience: one engine process at a time is the rule the queue exists to
// enforce, and a second lane for files would put two models on the GPU at once.
// A job is identified in the queue by its folder, exactly as a session is.

// fileOutcome is what a finished file job brings back to Update: the job as it
// now stands on disk and, on success, its transcript.
type fileOutcome struct {
	job  filejob.Job
	text string
}

type filesLoadedMsg struct {
	jobs []filejob.Job
	err  error
}

type filesAddedMsg struct {
	jobs []filejob.Job
	errs []string
	// quiet marks a paste that arrived outside the Files tab. It is only
	// acted on when it plainly was a dropped media file; otherwise it is
	// someone's stray clipboard and saying anything about it would be noise.
	quiet bool
}

type fileTextMsg struct {
	dir  string
	text string
	err  error
}

type fileRemovedMsg struct {
	dir  string
	name string
	err  error
}

type filesNoteMsg struct {
	note  string
	isErr bool
}

// fileLanguageChoices is what l cycles through after the configured language.
// "" asks the engine to detect the language itself.
var fileLanguageChoices = []string{"", "es", "en", "pt", "fr", "de", "it"}

// fileLanguage is the language a newly added file is transcribed with: the
// meeting language until the user picks another on this tab.
func (m Model) fileLanguage() string {
	if m.fileLangSet {
		return m.fileLang
	}
	return strings.TrimSpace(m.deps.Config.Transcription.Language)
}

func (m Model) nextFileLanguage() string {
	choices := []string{strings.TrimSpace(m.deps.Config.Transcription.Language)}
	for _, c := range fileLanguageChoices {
		if c != choices[0] {
			choices = append(choices, c)
		}
	}
	cur := m.fileLanguage()
	for i, c := range choices {
		if c == cur {
			return choices[(i+1)%len(choices)]
		}
	}
	return choices[0]
}

func (m Model) loadFileJobsCmd() tea.Cmd {
	root := m.deps.FilesRoot
	return func() tea.Msg {
		jobs, err := filejob.List(root)
		return filesLoadedMsg{jobs: jobs, err: err}
	}
}

func (m Model) handleFilesLoaded(msg filesLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m = m.setFileNote(msg.err.Error(), true)
		return m, nil
	}
	selected := ""
	if job, ok := m.selectedFile(); ok {
		selected = job.Dir
	}
	// A job that is running or queued may carry edits not written to disk yet
	// — the language chosen for a re-run is only saved when the run ends — so
	// the copy in memory wins over the one just read.
	jobs := msg.jobs
	for i := range jobs {
		if !m.fileJobBusy(jobs[i].Dir) {
			continue
		}
		if cur, ok := m.fileJob(jobs[i].Dir); ok {
			jobs[i] = cur
		}
	}
	m.fileJobs = jobs
	m.fileCursor = 0
	for i := range jobs {
		if jobs[i].Dir == selected {
			m.fileCursor = i
			break
		}
	}
	return m.syncFileText()
}

// addFilesCmd resolves pasted or typed text into files and registers a job for
// each. Everything here touches the disk, so none of it runs in Update.
func (m Model) addFilesCmd(text string, quiet bool) tea.Cmd {
	root := m.deps.FilesRoot
	lang := m.fileLanguage()
	return func() tea.Msg {
		paths := filejob.ParseDropped(text, filejob.IsFile)
		if quiet {
			if len(paths) == 0 {
				return filesAddedMsg{quiet: true}
			}
			for _, p := range paths {
				if !filejob.IsFile(p) || !filejob.LooksLikeMedia(p) {
					return filesAddedMsg{quiet: true}
				}
			}
		}
		msg := filesAddedMsg{quiet: quiet}
		for _, p := range paths {
			job, err := filejob.Create(root, p, lang, time.Now())
			if err != nil {
				msg.errs = append(msg.errs, err.Error())
				continue
			}
			msg.jobs = append(msg.jobs, job)
		}
		return msg
	}
}

func (m Model) handleFilesAdded(msg filesAddedMsg) (tea.Model, tea.Cmd) {
	if len(msg.jobs) == 0 {
		if msg.quiet {
			return m, nil
		}
		note := "that is not the path of a file"
		if len(msg.errs) > 0 {
			note = msg.errs[0]
		}
		return m.setFileNote(note, true), nil
	}

	var cmds []tea.Cmd
	if m.screen != ScreenFiles {
		model, cmd := m.switchScreen(ScreenFiles)
		m = model.(Model)
		cmds = append(cmds, cmd)
	}

	// Newest first, as the list is kept; the file dropped first runs first and
	// is the one left selected, so its transcript is what appears.
	jobs := make([]filejob.Job, 0, len(msg.jobs)+len(m.fileJobs))
	for i := len(msg.jobs) - 1; i >= 0; i-- {
		jobs = append(jobs, msg.jobs[i])
	}
	m.fileJobs = append(jobs, m.fileJobs...)
	m.fileCursor = len(msg.jobs) - 1
	m = m.clearFileText()

	for _, job := range msg.jobs {
		var cmd tea.Cmd
		m, cmd = m.enqueueTranscribe(job.Dir)
		cmds = append(cmds, cmd)
	}

	switch {
	case len(msg.errs) > 0:
		m = m.setFileNote(fmt.Sprintf("added %d, skipped %d — %s", len(msg.jobs), len(msg.errs), msg.errs[0]), true)
	case len(msg.jobs) == 1:
		m = m.setFileNote("added "+msg.jobs[0].Name, false)
	default:
		m = m.setFileNote(fmt.Sprintf("added %d files", len(msg.jobs)), false)
	}
	return m, tea.Batch(cmds...)
}

// handlePaste receives bracketed-paste text, which is how a terminal delivers
// a file dropped onto it.
func (m Model) handlePaste(msg tea.PasteMsg) (tea.Model, tea.Cmd) {
	if m.setupOpen || m.quitConfirmOpen || m.helpOpen {
		return m, nil
	}
	if m.screen == ScreenFiles && m.fileEditing {
		m.fileInput += sanitizeFileInput(msg.Content)
		return m, nil
	}
	// Config has text fields of its own; a paste there is not a dropped file.
	if m.screen == ScreenConfig {
		return m, nil
	}
	return m, m.addFilesCmd(msg.Content, m.screen != ScreenFiles)
}

func sanitizeFileInput(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' {
			return ' '
		}
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, s)
}

func (m Model) fileJob(dir string) (filejob.Job, bool) {
	if dir == "" {
		return filejob.Job{}, false
	}
	for _, j := range m.fileJobs {
		if j.Dir == dir {
			return j, true
		}
	}
	return filejob.Job{}, false
}

func (m Model) selectedFile() (filejob.Job, bool) {
	if m.fileCursor < 0 || m.fileCursor >= len(m.fileJobs) {
		return filejob.Job{}, false
	}
	return m.fileJobs[m.fileCursor], true
}

// fileJobBusy reports whether the job is running or waiting in the queue.
func (m Model) fileJobBusy(dir string) bool {
	if m.transcribeActive && m.transcribeSessionDir == dir {
		return true
	}
	return transcribeQueued(m.transcribeQueue, dir)
}

func (m Model) fileState(job filejob.Job) (components.FileState, int) {
	if m.transcribeActive && m.transcribeSessionDir == job.Dir {
		return components.FileRunning, 0
	}
	for i, dir := range m.transcribeQueue {
		if dir == job.Dir {
			return components.FileQueued, i + 1
		}
	}
	switch {
	case job.Error != "":
		return components.FileFailed, 0
	case job.Done:
		return components.FileDone, 0
	default:
		return components.FilePending, 0
	}
}

// updateFileJob replaces one job by folder. The slice is copied rather than
// written through: a previous Model value may still be holding the old one.
func (m Model) updateFileJob(job filejob.Job) Model {
	for i := range m.fileJobs {
		if m.fileJobs[i].Dir == job.Dir {
			jobs := append([]filejob.Job(nil), m.fileJobs...)
			jobs[i] = job
			m.fileJobs = jobs
			break
		}
	}
	return m
}

// applyFileOutcome folds a finished job back into the list.
func (m Model) applyFileOutcome(o *fileOutcome) Model {
	m = m.updateFileJob(o.job)
	if sel, ok := m.selectedFile(); ok && sel.Dir == o.job.Dir {
		if o.job.Done && o.job.Error == "" {
			m = m.setFileText(o.job.Dir, o.text)
		} else {
			m = m.clearFileText()
		}
	}
	return m
}

func (m Model) setFileNote(note string, isErr bool) Model {
	m.fileNote = note
	m.fileNoteErr = isErr
	if !isErr {
		m.markStatusTransient()
	}
	return m
}

// --- transcript text -------------------------------------------------------

func (m Model) clearFileText() Model {
	m.fileText = ""
	m.fileTextDir = ""
	m.fileLines = nil
	m.fileWords = 0
	m.fileScroll = 0
	return m
}

func (m Model) setFileText(dir, text string) Model {
	m.fileText = text
	m.fileTextDir = dir
	m.fileWords = len(strings.Fields(text))
	m.fileScroll = 0
	return m.rewrapFileText()
}

// rewrapFileText re-flows the transcript for the current width. It runs when
// the text or the terminal changes rather than in View: wrapping an hour of
// speech on every frame is exactly the per-frame work View is kept free of.
func (m Model) rewrapFileText() Model {
	if m.fileTextDir == "" {
		return m
	}
	m.fileLines = components.WrapText(m.fileText, m.fileTextWidth())
	return m.clampFileScroll()
}

func (m Model) fileTextWidth() int {
	return components.NewPanelLayout(m.width).FullWidth() - 4
}

func (m Model) clampFileScroll() Model {
	m.fileScroll = clampScroll(m.fileScroll, max(len(m.fileLines)-m.fileTextRows(), 0))
	return m
}

// syncFileText makes the loaded transcript match the selected job, reading it
// from disk through a command when it is not the one already held.
func (m Model) syncFileText() (Model, tea.Cmd) {
	job, ok := m.selectedFile()
	if !ok || !job.Done {
		return m.clearFileText(), nil
	}
	if m.fileTextDir == job.Dir {
		return m, nil
	}
	m = m.clearFileText()
	dir := job.Dir
	return m, func() tea.Msg {
		text, err := filejob.ReadTranscript(dir)
		return fileTextMsg{dir: dir, text: text, err: err}
	}
}

func (m Model) handleFileText(msg fileTextMsg) (tea.Model, tea.Cmd) {
	job, ok := m.selectedFile()
	if !ok || job.Dir != msg.dir {
		return m, nil // the selection moved on while the file was being read
	}
	if msg.err != nil {
		return m.setFileNote("could not read the transcript: "+msg.err.Error(), true), nil
	}
	return m.setFileText(msg.dir, msg.text), nil
}

// --- layout ----------------------------------------------------------------

// filesChromeRows is everything on the Files screen that is not the list or
// the transcript: header, tab bar, the blank line under it, the footer's rule
// and hints (5), the drop panel (5), the gaps between the three panels (2) and
// the transcript panel's borders and status line (3).
const filesChromeRows = 15

func (m Model) fileListRows() int {
	n := len(m.fileJobs)
	if n == 0 {
		return 0
	}
	limit := 5
	if m.height > 0 {
		limit = min(max((m.height-filesChromeRows)/3, 3), 8)
	}
	return min(n, limit)
}

// fileListWindow is the slice of the list on screen. It is derived from the
// cursor alone, so View needs no scroll state of its own.
func (m Model) fileListWindow() (first, count int) {
	count = m.fileListRows()
	if m.fileCursor >= count {
		first = m.fileCursor - count + 1
	}
	return first, count
}

func (m Model) fileTextRows() int {
	if m.height <= 0 {
		return 10
	}
	list := 3 // an empty list is a one-line panel
	if n := m.fileListRows(); n > 0 {
		list = n + 3 // header row and borders
	}
	return max(m.height-filesChromeRows-list, 3)
}

// --- keys ------------------------------------------------------------------

func (m Model) moveFileCursor(delta int) (Model, tea.Cmd) {
	if len(m.fileJobs) == 0 {
		return m, nil
	}
	next := min(max(m.fileCursor+delta, 0), len(m.fileJobs)-1)
	if next == m.fileCursor {
		return m, nil
	}
	m.fileCursor = next
	return m.syncFileText()
}

// fileWheelStep is how many lines one wheel notch scrolls the transcript.
const fileWheelStep = 3

func (m Model) scrollFileText(delta int) Model {
	m.fileScroll += delta
	return m.clampFileScroll()
}

func (m Model) handleFilesKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if key != "d" && key != "delete" {
		m.fileRemoveArmed = ""
	}
	// An error stays until the next thing the user does.
	if m.fileNoteErr {
		m.fileNote, m.fileNoteErr = "", false
	}

	switch key {
	case "up", "k":
		return m.moveFileCursor(-1)
	case "down", "j":
		return m.moveFileCursor(1)
	case "pgup":
		return m.scrollFileText(-m.fileTextRows()), nil
	case "pgdown", "space":
		return m.scrollFileText(m.fileTextRows()), nil
	case "home", "g":
		return m.scrollFileText(-len(m.fileLines)), nil
	case "end", "G":
		return m.scrollFileText(len(m.fileLines)), nil
	case "a":
		m.fileEditing = true
		m.fileInput = ""
		m.fileNote = ""
		return m, nil
	case "c", "y":
		return m.copyFileText()
	case "t":
		return m.retranscribeFile()
	case "s":
		if m.transcribeActive {
			return m.stopTranscribe()
		}
		return m, nil
	case "l":
		m.fileLang = m.nextFileLanguage()
		m.fileLangSet = true
		return m, nil
	case "o":
		return m, m.openFileFolderCmd()
	case "d", "delete":
		return m.removeFile()
	case "R":
		return m, m.loadFileJobsCmd()
	case "S":
		return m.openSetupWizard(), nil
	}
	return m, nil
}

// handleFilesInputKey owns the keyboard while a path is being typed, so that
// "q" in a file name is a letter and not the quit binding.
func (m Model) handleFilesInputKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.fileEditing = false
		m.fileInput = ""
		return m, nil
	case "enter":
		text := strings.TrimSpace(m.fileInput)
		m.fileEditing = false
		m.fileInput = ""
		if text == "" {
			return m, nil
		}
		return m, m.addFilesCmd(text, false)
	case "backspace":
		if r := []rune(m.fileInput); len(r) > 0 {
			m.fileInput = string(r[:len(r)-1])
		}
		return m, nil
	case "ctrl+u":
		m.fileInput = ""
		return m, nil
	}
	if text := msg.Key().Text; text != "" {
		m.fileInput += sanitizeFileInput(text)
	}
	return m, nil
}

func (m Model) copyFileText() (tea.Model, tea.Cmd) {
	job, ok := m.selectedFile()
	if !ok {
		return m.setFileNote("nothing to copy — add a file first", true), nil
	}
	if m.fileTextDir != job.Dir || strings.TrimSpace(m.fileText) == "" {
		return m.setFileNote("nothing to copy yet — "+job.Name+" has no transcript", true), nil
	}
	text := m.fileText
	m = m.setFileNote(fmt.Sprintf("copied %d words to the clipboard", m.fileWords), false)
	// Both routes, because neither is universal: OSC 52 asks the terminal to
	// set the clipboard and works over SSH, but some terminals and multiplexers
	// ignore it; a native tool always works where one is installed.
	return m, tea.Batch(tea.SetClipboard(text), nativeClipboardCmd(text))
}

// nativeClipboardCmd pipes text to the desktop's own clipboard tool when there
// is one. Missing tools are not an error: OSC 52 is still on its way.
func nativeClipboardCmd(text string) tea.Cmd {
	return func() tea.Msg {
		name, args := clipboardTool()
		if name == "" {
			return nil
		}
		cmd := exec.Command(name, args...)
		cmd.Stdin = strings.NewReader(text)
		_ = cmd.Run()
		return nil
	}
}

func clipboardTool() (string, []string) {
	// Linux only. clip.exe reads its input in the console code page and would
	// turn every accented character into mojibake; Windows Terminal honours
	// OSC 52, which carries UTF-8 intact.
	if runtime.GOOS != "linux" {
		return "", nil
	}
	candidates := []struct {
		name string
		args []string
	}{
		{"xclip", []string{"-selection", "clipboard"}},
		{"xsel", []string{"--clipboard", "--input"}},
	}
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		candidates = append([]struct {
			name string
			args []string
		}{{"wl-copy", nil}}, candidates...)
	}
	for _, c := range candidates {
		if path, err := exec.LookPath(c.name); err == nil {
			return path, c.args
		}
	}
	return "", nil
}

// retranscribeFile runs the selected file again with the language now chosen
// on the tab — the way to fix a run that guessed the language wrong.
func (m Model) retranscribeFile() (tea.Model, tea.Cmd) {
	job, ok := m.selectedFile()
	if !ok {
		return m, nil
	}
	if m.fileJobBusy(job.Dir) {
		return m, nil
	}
	job.Language = m.fileLanguage()
	job.Error = ""
	m = m.updateFileJob(job)
	return m.enqueueTranscribe(job.Dir)
}

// removeFile forgets the selected job. It asks twice: the transcript goes with
// it, and for a long recording that is minutes of GPU time.
func (m Model) removeFile() (tea.Model, tea.Cmd) {
	job, ok := m.selectedFile()
	if !ok {
		return m, nil
	}
	if m.transcribeActive && m.transcribeSessionDir == job.Dir {
		return m.setFileNote("it is being transcribed — press s to stop it first", true), nil
	}
	if m.fileRemoveArmed != job.Dir {
		m.fileRemoveArmed = job.Dir
		m.fileNote = "press d again to remove " + job.Name + " and its transcript — the original file stays"
		m.fileNoteErr = true
		return m, nil
	}
	m.fileRemoveArmed = ""
	m.fileNote, m.fileNoteErr = "", false
	// A queued job must leave the queue too, or it would run with no folder.
	queue := make([]string, 0, len(m.transcribeQueue))
	for _, dir := range m.transcribeQueue {
		if dir != job.Dir {
			queue = append(queue, dir)
		}
	}
	m.transcribeQueue = queue

	root := m.deps.FilesRoot
	return m, func() tea.Msg {
		return fileRemovedMsg{dir: job.Dir, name: job.Name, err: filejob.Remove(root, job.Dir)}
	}
}

func (m Model) handleFileRemoved(msg fileRemovedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		return m.setFileNote(msg.err.Error(), true), nil
	}
	jobs := make([]filejob.Job, 0, len(m.fileJobs))
	for _, j := range m.fileJobs {
		if j.Dir != msg.dir {
			jobs = append(jobs, j)
		}
	}
	m.fileJobs = jobs
	m.fileCursor = min(m.fileCursor, max(len(jobs)-1, 0))
	m = m.setFileNote("removed "+msg.name, false)
	return m.syncFileText()
}

func (m Model) openFileFolderCmd() tea.Cmd {
	job, ok := m.selectedFile()
	if !ok {
		return nil
	}
	opener := m.deps.Opener
	cfg := m.deps.Config.Desktop
	return func() tea.Msg {
		if err := opener.Open(job.Dir, cfg, open.KindFolder); err != nil {
			return filesNoteMsg{note: fmt.Sprintf("open %s: %v", job.Dir, err), isErr: true}
		}
		return nil
	}
}

// --- view ------------------------------------------------------------------

func (m Model) filesView() components.FilesView {
	first, count := m.fileListWindow()
	rows := make([]components.FileRow, 0, count)
	for i := first; i < first+count && i < len(m.fileJobs); i++ {
		rows = append(rows, m.fileRow(m.fileJobs[i]))
	}

	v := components.FilesView{
		Width:    m.width,
		Rows:     rows,
		Cursor:   m.fileCursor - first,
		First:    first,
		Total:    len(m.fileJobs),
		Language: m.fileLanguage(),
		Engine:   m.deps.Config.Transcription.Backend,
		Root:     m.filesRootLabel,
		Editing:  m.fileEditing,
		Input:    m.fileInput,
		Note:     m.fileNote,
		NoteErr:  m.fileNoteErr,
		ETA:      m.transcribeETA,
		Blink:    m.transcribeBlink,
		Log:      m.transcribeLog,
		TextRows: m.fileTextRows(),
	}
	if job, ok := m.selectedFile(); ok {
		v.HasSelection = true
		v.Selected = m.fileRow(job)
		v.SelError = job.Error
		if m.fileTextDir == job.Dir {
			end := min(m.fileScroll+v.TextRows, len(m.fileLines))
			start := min(m.fileScroll, end)
			v.Lines = m.fileLines[start:end]
			v.Scroll = start
			v.TotalLines = len(m.fileLines)
			v.Words = m.fileWords
		}
	}
	return v
}

func (m Model) fileRow(job filejob.Job) components.FileRow {
	state, pos := m.fileState(job)
	row := components.FileRow{
		Name:     job.Name,
		Added:    job.AddedAt.Local().Format("Jan 02 15:04"),
		Language: job.Language,
		State:    state,
		QueuePos: pos,
	}
	if d := job.Duration(); d > 0 {
		row.Duration = formatFileDuration(d)
	}
	if state == components.FileRunning {
		row.Percent = m.transcribePercent
	}
	return row
}

func formatFileDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h, mnt, s := int(d/time.Hour), int(d/time.Minute)%60, int(d/time.Second)%60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, mnt, s)
	}
	return fmt.Sprintf("%d:%02d", mnt, s)
}

func (m Model) filesFooter() components.FilesFooterMode {
	switch {
	case m.fileEditing:
		return components.FilesFooterEditing
	case m.transcribeActive:
		return components.FilesFooterTranscribing
	case len(m.fileJobs) == 0:
		return components.FilesFooterEmpty
	default:
		return components.FilesFooterNormal
	}
}

// filesRootDisplay shortens the jobs folder for the screen, home as ~.
func filesRootDisplay(root string) string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		if rel, err := filepath.Rel(home, root); err == nil && !strings.HasPrefix(rel, "..") {
			return filepath.Join("~", rel)
		}
	}
	return root
}
