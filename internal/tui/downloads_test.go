package tui

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"anoted/internal/config"
	"anoted/internal/transcribe"
	"anoted/internal/tui/components"
	tea "charm.land/bubbletea/v2"
)

type fakeDownloads struct {
	items   []transcribe.Download
	removed []string
}

func (f *fakeDownloads) List(config.TranscriptionConfig) []transcribe.Download {
	return append([]transcribe.Download(nil), f.items...)
}

func (f *fakeDownloads) Remove(_ config.TranscriptionConfig, id string) (transcribe.Download, error) {
	for i, d := range f.items {
		if d.ID == id {
			if d.InUse {
				return d, transcribe.ErrDownloadInUse
			}
			f.removed = append(f.removed, id)
			f.items = append(f.items[:i], f.items[i+1:]...)
			return d, nil
		}
	}
	return transcribe.Download{}, errors.New("no such download")
}

func modelsScreen(t *testing.T) (Model, *fakeDownloads) {
	t.Helper()
	store := &fakeDownloads{items: []transcribe.Download{
		{ID: "whisper-runtime", Label: "Whisper runtime", Path: "/data/anoted/whisper-venv", Bytes: 7 << 30},
		{ID: "parakeet-model", Label: "Parakeet TDT 0.6B v3 model", Path: "/data/anoted/models/p", Bytes: 2 << 30, InUse: true},
	}}
	cfg := config.Default()
	cfg.Transcription.Backend = transcribe.BackendParakeet
	m := Model{
		screen: ScreenConfig,
		width:  110,
		height: 40,
		scroll: newSessionScroll(),
		cache:  &viewCache{},
		deps: Deps{
			Config:     cfg,
			ConfigPath: filepath.Join(t.TempDir(), "config.yaml"),
			Downloads:  store,
		},
	}
	// Models is the last section: one step left from General.
	next, cmd := m.handleConfigKey(tea.KeyPressMsg{Code: tea.KeyLeft})
	m = run(t, next.(Model), cmd)
	if m.configSection != modelsSection || !m.downloadsScanned || len(m.downloads) != 2 {
		t.Fatalf("section=%d scanned=%v downloads=%d", m.configSection, m.downloadsScanned, len(m.downloads))
	}
	return m, store
}

func press(t *testing.T, m Model, code rune) (Model, tea.Cmd) {
	t.Helper()
	next, cmd := m.handleConfigKey(tea.KeyPressMsg{Code: code, Text: string(code)})
	return next.(Model), cmd
}

func TestModelsSectionListsDownloads(t *testing.T) {
	m, _ := modelsScreen(t)
	fields := m.currentCfgFields()
	if len(fields) != 3 {
		t.Fatalf("%d fields", len(fields))
	}
	if v := cfgFieldValue(fields[0], m.deps.Config); !strings.Contains(v, "9.0 GB") || !strings.Contains(v, "7.0 GB can be freed") {
		t.Fatalf("summary %q", v)
	}
	if v := cfgFieldValue(fields[2], m.deps.Config); !strings.Contains(v, "in use") {
		t.Fatalf("in-use row %q", v)
	}
	if m.configFooter() != components.ConfigFooterModels {
		t.Fatalf("footer mode %d", m.configFooter())
	}
}

func TestDeleteDownloadNeedsTwoPresses(t *testing.T) {
	m, store := modelsScreen(t)
	m.configCursor = 1 // Whisper runtime

	m, cmd := press(t, m, 'd')
	if cmd != nil || len(store.removed) != 0 {
		t.Fatal("deleted on the first press")
	}
	if v := cfgFieldValue(m.currentCfgFields()[1], m.deps.Config); !strings.Contains(v, "press d again") {
		t.Fatalf("armed row %q", v)
	}

	// Any other key disarms it.
	m, _ = press(t, m, 'j')
	m, _ = press(t, m, 'k')
	if m.downloadArmed != "" {
		t.Fatal("moving the cursor kept the delete armed")
	}

	m, _ = press(t, m, 'd')
	m, cmd = press(t, m, 'd')
	if cmd == nil || m.downloadRemoving != "whisper-runtime" {
		t.Fatal("second press did not delete")
	}
	next, follow := m.Update(cmd())
	m = next.(Model)
	if strings.Join(store.removed, ",") != "whisper-runtime" {
		t.Fatalf("removed %v", store.removed)
	}
	if !strings.Contains(m.configSavedMsg, "freed 7.0 GB") || m.downloadRemoving != "" {
		t.Fatalf("note %q removing %q", m.configSavedMsg, m.downloadRemoving)
	}
	if follow == nil {
		t.Fatal("no rescan after deleting")
	}
}

func TestDeleteRefusesWhatIsInUse(t *testing.T) {
	m, store := modelsScreen(t)
	m.configCursor = 2 // Parakeet model, in use
	m, cmd := press(t, m, 'd')
	m, cmd2 := press(t, m, 'd')
	if cmd != nil || cmd2 != nil || len(store.removed) != 0 {
		t.Fatal("deleted what the backend runs")
	}
	if !strings.Contains(m.configErr, "switch backend first") {
		t.Fatalf("err %q", m.configErr)
	}
}

func TestDeleteWaitsForTranscription(t *testing.T) {
	m, store := modelsScreen(t)
	m.configCursor = 1
	m.transcribeActive = true
	m, _ = press(t, m, 'd')
	_, cmd := press(t, m, 'd')
	if cmd != nil || len(store.removed) != 0 || m.configErr == "" {
		t.Fatal("deleted during a transcription")
	}
}

func TestDeletingTheVenvClearsAStaleBinary(t *testing.T) {
	m, _ := modelsScreen(t)
	m.deps.Config.Transcription.Binary = "/data/anoted/whisper-venv/bin/whisper"
	_, cmd := m.handleDownloadRemoved(downloadRemovedMsg{download: m.downloads[0].Download})

	var saved *configMenuSaveMsg
	for _, c := range cmd().(tea.BatchMsg) {
		if c == nil {
			continue
		}
		if msg, ok := c().(configMenuSaveMsg); ok {
			saved = &msg
		}
	}
	if saved == nil || saved.err != nil {
		t.Fatalf("config not saved: %+v", saved)
	}
	if saved.cfg.Transcription.Binary != "" || !strings.Contains(saved.note, "freed") {
		t.Fatalf("saved %+v", saved)
	}
}

func TestModelsViewIsPure(t *testing.T) {
	m, _ := modelsScreen(t)
	m.configCursor = 1
	m, _ = press(t, m, 'd')
	first, second := m.View(), m.View()
	if first.Content != second.Content {
		t.Fatal("Models view is not a pure function of the Model")
	}
	if !strings.Contains(first.Content, "press d again") || !strings.Contains(first.Content, "MODELS") {
		t.Fatal("armed delete not shown")
	}
}

// Config owns its keys, so every tab number has to be let through explicitly;
// "4" was missed when the Files tab was added and left Config a dead end.
func TestFilesTabKeyLeavesConfig(t *testing.T) {
	m, _ := modelsScreen(t)
	m.deps.FilesRoot = t.TempDir()
	next, _ := m.Update(tea.KeyPressMsg{Code: '4', Text: "4"})
	if got := next.(Model).screen; got != ScreenFiles {
		t.Fatalf("4 from Config: screen %s, want %s", got, ScreenFiles)
	}
}
