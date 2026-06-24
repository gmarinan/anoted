package autostart

import "anoted/internal/config"

// EntryFromConfig builds a launch entry using desktop autostart settings.
func EntryFromConfig(cfg config.Config) (Entry, error) {
	entry, err := DefaultEntry()
	if err != nil {
		return Entry{}, err
	}
	entry.WMClass = cfg.Desktop.WMClass
	entry.TerminalCommand = append([]string(nil), cfg.Desktop.AutostartTerminal...)
	if entry.WMClass == "" {
		entry.WMClass = "anoted"
	}
	return entry, nil
}
