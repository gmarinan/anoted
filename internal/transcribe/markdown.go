package transcribe

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"anoted/internal/config"
	"anoted/internal/recorder"
	"anoted/internal/session"
)

type meetingFrontmatter struct {
	StartedAt    string   `yaml:"started_at,omitempty"`
	EndedAt      string   `yaml:"ended_at,omitempty"`
	Duration     string   `yaml:"duration,omitempty"`
	Provider     string   `yaml:"provider,omitempty"`
	Platform     string   `yaml:"platform,omitempty"`
	Backend      string   `yaml:"backend,omitempty"`
	AutoRecord   bool     `yaml:"auto_record,omitempty"`
	Manual       bool     `yaml:"manual,omitempty"`
	SystemDevice string   `yaml:"system_device,omitempty"`
	MicDevice    string   `yaml:"mic_device,omitempty"`
	Date         string   `yaml:"date,omitempty"`
	Tags         []string `yaml:"tags,omitempty"`
	CSSClasses   []string `yaml:"cssclasses,omitempty"`
}

// WriteMeetingMarkdown creates an Obsidian-style note with YAML frontmatter and transcript body.
func WriteMeetingMarkdown(sessionDir, outDir string, cfg config.TranscriptionConfig) error {
	meta, err := session.ReadMetadataFile(sessionDir)
	if err != nil {
		return fmt.Errorf("read session metadata: %w", err)
	}

	fileBase := outputFileBase(cfg, sessionDir)
	txtPath := filepath.Join(outDir, fileBase+".txt")
	body, err := os.ReadFile(txtPath)
	if err != nil {
		return fmt.Errorf("read transcript for markdown: %w", err)
	}

	fm := buildFrontmatter(meta, cfg, sessionDir)
	fmYAML, err := yaml.Marshal(&fm)
	if err != nil {
		return fmt.Errorf("marshal frontmatter: %w", err)
	}

	var b strings.Builder
	b.WriteString("---\n")
	b.Write(fmYAML)
	b.WriteString("---\n\n")
	b.Write(body)

	outPath := filepath.Join(outDir, markdownFilename(cfg, sessionDir))
	if err := os.WriteFile(outPath, []byte(b.String()), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", outPath, err)
	}
	return nil
}

func buildFrontmatter(meta session.Metadata, cfg config.TranscriptionConfig, sessionDir string) meetingFrontmatter {
	started := meta.StartedAt.Local()
	fm := meetingFrontmatter{
		Provider:   string(meta.Provider),
		Platform:   meta.Platform,
		Backend:    meta.Backend,
		AutoRecord: meta.AutoRecord,
		Manual:     meta.Manual,
		Tags:       append([]string(nil), cfg.Markdown.Tags...),
		CSSClasses: append([]string(nil), cfg.Markdown.CSSClasses...),
	}
	if !meta.StartedAt.IsZero() {
		fm.StartedAt = meta.StartedAt.Local().Format(time.RFC3339)
		fm.Date = started.Format("2006-01-02")
	}
	if !meta.EndedAt.IsZero() {
		fm.EndedAt = meta.EndedAt.Local().Format(time.RFC3339)
	}
	if meta.Duration != "" {
		fm.Duration = meta.Duration
	} else if !meta.StartedAt.IsZero() && !meta.EndedAt.IsZero() {
		fm.Duration = meta.EndedAt.Sub(meta.StartedAt).Round(time.Second).String()
	} else if sessionDir != "" {
		if dur, err := AudioDuration(filepath.Join(sessionDir, recorder.SessionAudioFile)); err == nil && dur > 0 {
			fm.Duration = dur.Round(time.Second).String()
			if fm.EndedAt == "" && !meta.StartedAt.IsZero() {
				fm.EndedAt = meta.StartedAt.Add(dur).Local().Format(time.RFC3339)
			}
		}
	}
	if meta.SystemDevice != "" {
		fm.SystemDevice = truncateDevice(meta.SystemDevice, 80)
	}
	if meta.MicDevice != "" {
		fm.MicDevice = truncateDevice(meta.MicDevice, 80)
	}
	if tag := providerTag(meta.Provider); tag != "" {
		fm.Tags = appendUnique(fm.Tags, tag)
	}
	if cfg.Markdown.MarkdownWeekdayClassEnabled() && !started.IsZero() {
		fm.CSSClasses = appendUnique(fm.CSSClasses, strings.ToLower(started.Weekday().String()))
	}
	return fm
}

func providerTag(p session.Provider) string {
	switch p {
	case session.ProviderGoogleMeet:
		return "google-meet"
	case session.ProviderTeams:
		return "teams"
	default:
		return ""
	}
}

func appendUnique(items []string, v string) []string {
	for _, existing := range items {
		if existing == v {
			return items
		}
	}
	return append(items, v)
}

func truncateDevice(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

// ExtractMarkdownBody returns transcript text from a markdown file (content after frontmatter).
func ExtractMarkdownBody(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	content := string(data)
	if !strings.HasPrefix(content, "---") {
		return strings.TrimSpace(content), nil
	}
	rest := content[3:]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return strings.TrimSpace(content), nil
	}
	body := strings.TrimPrefix(rest[end+4:], "\n")
	return strings.TrimSpace(body), nil
}
