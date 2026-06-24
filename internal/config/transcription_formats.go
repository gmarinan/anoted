package config

import "strings"

// Valid transcription output format identifiers.
const (
	OutputFormatTXT  = "txt"
	OutputFormatSRT  = "srt"
	OutputFormatVTT  = "vtt"
	OutputFormatJSON = "json"
	OutputFormatMD   = "md"
)

var validOutputFormats = map[string]bool{
	OutputFormatTXT:  true,
	OutputFormatSRT:  true,
	OutputFormatVTT:  true,
	OutputFormatJSON: true,
	OutputFormatMD:   true,
}

// NormalizeOutputFormats filters unknown values and ensures at least txt.
func NormalizeOutputFormats(formats []string) []string {
	var out []string
	seen := make(map[string]bool, len(formats))
	for _, f := range formats {
		f = strings.ToLower(strings.TrimSpace(f))
		if !validOutputFormats[f] || seen[f] {
			continue
		}
		seen[f] = true
		out = append(out, f)
	}
	if len(out) == 0 {
		return []string{OutputFormatTXT}
	}
	return out
}

// WantsMarkdown reports whether md is among the output formats.
func WantsMarkdown(formats []string) bool {
	for _, f := range formats {
		if f == OutputFormatMD {
			return true
		}
	}
	return false
}
