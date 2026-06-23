package session

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Provider identifies the detected meeting platform.
type Provider string

const (
	ProviderGoogleMeet Provider = "google_meet"
	ProviderTeams      Provider = "teams"
	ProviderUnknown    Provider = "unknown"
)

// Status is the lifecycle state of a recording session.
type Status string

const (
	StatusActive  Status = "active"
	StatusStopped Status = "stopped"
	StatusError   Status = "error"
)

// Metadata is persisted alongside audio files.
type Metadata struct {
	StartedAt    time.Time `json:"started_at"`
	EndedAt      time.Time `json:"ended_at,omitempty"`
	Duration     string    `json:"duration,omitempty"`
	Provider     Provider  `json:"provider"`
	Platform     string    `json:"platform"`
	Backend      string    `json:"backend"`
	SystemDevice       string    `json:"system_device,omitempty"`
	MicDevice          string    `json:"mic_device,omitempty"`
	OutputSampleRate   int       `json:"output_sample_rate,omitempty"`
	Channels           int       `json:"channels,omitempty"`
	SystemInternalRate int       `json:"system_internal_rate,omitempty"`
	MicInternalRate    int       `json:"mic_internal_rate,omitempty"`
	AutoRecord         bool      `json:"auto_record"`
	Manual       bool      `json:"manual"`
}

// Record is a stored session row.
type Record struct {
	ID        int64
	Dir       string
	Provider  Provider
	Platform  string
	Backend   string
	StartedAt time.Time
	EndedAt   time.Time
	Status    Status
	Metadata  Metadata
}

// Store persists session metadata.
type Store interface {
	Open() error
	Close() error
	Create(rec Record) (int64, error)
	Update(rec Record) error
	Get(id int64) (Record, error)
	List(limit int) ([]Record, error)
	Delete(id int64) error
}

// Remove deletes the session directory and database row.
func Remove(store Store, rec Record) error {
	if rec.Dir != "" {
		if err := os.RemoveAll(rec.Dir); err != nil {
			return fmt.Errorf("remove session dir %s: %w", rec.Dir, err)
		}
	}
	if store != nil {
		if err := store.Delete(rec.ID); err != nil {
			return fmt.Errorf("delete session row: %w", err)
		}
	}
	return nil
}

// FormatLocalTime formats t in the system local timezone.
func FormatLocalTime(t time.Time, layout string) string {
	if t.IsZero() {
		return ""
	}
	return t.Local().Format(layout)
}

// WriteMetadataFile writes metadata.json into a session directory.
func WriteMetadataFile(dir string, meta Metadata) error {
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return writeFile(dir, "metadata.json", data)
}
