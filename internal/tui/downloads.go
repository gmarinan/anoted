package tui

import (
	"fmt"

	"anoted/internal/config"
	"anoted/internal/retention"
	"anoted/internal/transcribe"
	tea "charm.land/bubbletea/v2"
)

// modelsSection is the Config section listing downloaded runtimes and models.
const modelsSection = 7

// downloadEntry is a download plus what the Models section shows for its path,
// worked out when the scan lands so View does no lookups.
type downloadEntry struct {
	transcribe.Download
	display string
}

type downloadsScannedMsg struct {
	downloads []transcribe.Download
}

type downloadRemovedMsg struct {
	download transcribe.Download
	err      error
}

// scanDownloadsCmd sizes everything on disk. It walks multi-gigabyte venvs and
// may start Python to resolve the engine, so it never runs inside Update.
func (m Model) scanDownloadsCmd() tea.Cmd {
	tcfg := m.deps.Config.Transcription
	store := m.downloadStore()
	return func() tea.Msg {
		return downloadsScannedMsg{downloads: store.List(tcfg)}
	}
}

func (m Model) downloadStore() DownloadStore {
	if m.deps.Downloads != nil {
		return m.deps.Downloads
	}
	return systemDownloads{}
}

// enterModelsSection rescans whenever the section comes into view: sizes and
// what is in use change with installs, transcriptions and backend switches.
func (m Model) enterModelsSection() (Model, tea.Cmd) {
	if m.configSection != modelsSection || m.downloadsScanning {
		return m, nil
	}
	m.downloadsScanning = true
	return m, m.scanDownloadsCmd()
}

func (m Model) handleDownloadsScanned(msg downloadsScannedMsg) (tea.Model, tea.Cmd) {
	m.downloadsScanning = false
	m.downloadsScanned = true
	m.downloads = make([]downloadEntry, 0, len(msg.downloads))
	armedStill := false
	for _, d := range msg.downloads {
		m.downloads = append(m.downloads, downloadEntry{Download: d, display: filesRootDisplay(d.Path)})
		if d.ID == m.downloadArmed && !d.InUse {
			armedStill = true
		}
	}
	if !armedStill {
		m.downloadArmed = ""
	}
	if m.screen == ScreenConfig && m.configSection == modelsSection {
		m = m.clampConfigCursor()
	}
	return m, nil
}

func (m Model) downloadByID(id string) (downloadEntry, bool) {
	for _, d := range m.downloads {
		if d.ID == id {
			return d, true
		}
	}
	return downloadEntry{}, false
}

// pressDeleteDownload arms the selected download on the first press and
// deletes it on the second, the way removing a file job works.
func (m Model) pressDeleteDownload(f cfgField) (tea.Model, tea.Cmd) {
	d, ok := m.downloadByID(f.downloadID)
	if !ok || m.downloadRemoving != "" {
		return m, nil
	}
	m.configSavedMsg = ""
	switch {
	case d.InUse:
		m.configErr = fmt.Sprintf("in use by %s — switch backend first", m.deps.Config.Transcription.Backend)
		return m, nil
	case m.whisperInstall.Active || m.gpuInstall.Active:
		m.configErr = "an install is running — wait for it to finish"
		return m, nil
	case m.transcribeActive:
		// What is in use was worked out from the config as it is now; a run
		// started before a backend switch may still be reading this.
		m.configErr = "a transcription is running — wait for it to finish"
		return m, nil
	}
	m.configErr = ""
	if m.downloadArmed != d.ID {
		m.downloadArmed = d.ID
		return m, nil
	}
	m.downloadArmed = ""
	m.downloadRemoving = d.ID
	tcfg := m.deps.Config.Transcription
	id := d.ID
	store := m.downloadStore()
	return m, func() tea.Msg {
		removed, err := store.Remove(tcfg, id)
		return downloadRemovedMsg{download: removed, err: err}
	}
}

func (m Model) handleDownloadRemoved(msg downloadRemovedMsg) (tea.Model, tea.Cmd) {
	m.downloadRemoving = ""
	if msg.err != nil {
		m.configErr = msg.err.Error()
		m.configSavedMsg = ""
		m.downloadsScanning = true
		return m, m.scanDownloadsCmd()
	}
	d := msg.download
	note := fmt.Sprintf("freed %s — %s deleted", retention.FormatBytes(d.Bytes), d.Label)
	m.configErr = ""
	m.configSavedMsg = note
	m.markStatusTransient()

	cfg := m.deps.Config
	cmds := []tea.Cmd{refreshDoctorCapsCmd(cfg)}
	if d.Contains(cfg.Transcription.Binary) {
		// transcription.binary pointing into a deleted venv would make
		// switching back to Whisper fail with a bare "no such file".
		cfg.Transcription.Binary = ""
		cmds = append(cmds, m.saveConfigNoteCmd(cfg, note))
	}
	m.downloadsScanning = true
	cmds = append(cmds, m.scanDownloadsCmd())
	return m, tea.Batch(cmds...)
}

// saveConfigNoteCmd saves cfg and shows note instead of the usual "saved".
func (m Model) saveConfigNoteCmd(cfg config.Config, note string) tea.Cmd {
	path := m.deps.ConfigPath
	return func() tea.Msg {
		if err := config.Save(path, cfg); err != nil {
			return configMenuSaveMsg{err: err}
		}
		return configMenuSaveMsg{cfg: cfg, note: note}
	}
}

func modelsCfgFields(env envFacts) []cfgField {
	summary := func(v string) []cfgField {
		return []cfgField{{
			label: "on_disk",
			kind:  fieldReadonly,
			get:   func(config.Config) string { return v },
		}}
	}
	if !env.DownloadsScanned {
		return summary("scanning…")
	}
	if len(env.Downloads) == 0 {
		return summary("no downloaded runtimes or models")
	}
	var total, freeable int64
	for _, d := range env.Downloads {
		total += d.Bytes
		if !d.InUse {
			freeable += d.Bytes
		}
	}
	fields := summary(fmt.Sprintf("%s · %s can be freed", retention.FormatBytes(total), retention.FormatBytes(freeable)))
	for _, d := range env.Downloads {
		value := retention.FormatBytes(d.Bytes)
		switch {
		case env.DownloadRemoving == d.ID:
			value += " · deleting…"
		case env.DownloadArmed == d.ID:
			value += " · press d again to delete"
		case d.InUse:
			value += " · in use"
		case d.Shared:
			value += " · shared cache"
		}
		value += " · " + d.display
		inUse := d.InUse
		fields = append(fields, cfgField{
			label:      d.Label,
			kind:       fieldDownload,
			downloadID: d.ID,
			editable:   func(config.Config) bool { return !inUse },
			get:        func(config.Config) string { return value },
		})
	}
	return fields
}
