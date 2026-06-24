package transcribe

import (
	"os"
	"path/filepath"
	"strings"

	"anoted/internal/config"
)

var whisperNativeFormats = []string{
	config.OutputFormatTXT,
	config.OutputFormatSRT,
	config.OutputFormatVTT,
	config.OutputFormatJSON,
}

// nativeOutputFormats returns whisper-native formats requested by the user.
func nativeOutputFormats(formats []string) []string {
	want := make(map[string]bool, len(formats))
	for _, f := range formats {
		want[f] = true
	}
	var out []string
	for _, f := range whisperNativeFormats {
		if want[f] {
			out = append(out, f)
		}
	}
	return out
}

// effectiveWhisperFormats returns formats to ask whisper for, including temporary txt for md.
func effectiveWhisperFormats(formats []string) []string {
	native := nativeOutputFormats(formats)
	if config.WantsMarkdown(formats) && !containsFormat(native, config.OutputFormatTXT) {
		native = append([]string{config.OutputFormatTXT}, native...)
	}
	if len(native) == 0 {
		return []string{config.OutputFormatTXT}
	}
	return native
}

// whisperOutputFormatArg picks --output_format for openai-whisper CLI.
func whisperOutputFormatArg(formats []string) string {
	native := effectiveWhisperFormats(formats)
	// Strip to only user-native for arg when md forced txt internally - actually effective includes forced txt
	// For CLI: if only need txt (md-only), use txt. If multiple, use all then prune.
	if len(native) == 1 {
		return native[0]
	}
	return "all"
}

func containsFormat(formats []string, name string) bool {
	for _, f := range formats {
		if f == name {
			return true
		}
	}
	return false
}

// keepNativeFormats returns native formats to retain after transcription.
func keepNativeFormats(formats []string) map[string]bool {
	keep := make(map[string]bool)
	for _, f := range nativeOutputFormats(formats) {
		keep[f] = true
	}
	return keep
}

// pruneTranscriptFiles removes transcript.* files not in the keep set.
func pruneTranscriptFiles(dir string, keep map[string]bool, fileBase string) {
	for _, ext := range []string{".txt", ".srt", ".vtt", ".json"} {
		format := strings.TrimPrefix(ext, ".")
		if keep[format] {
			continue
		}
		_ = os.Remove(filepath.Join(dir, fileBase+ext))
	}
}

// removeTemporaryTxt removes the txt file when generated only for markdown.
func removeTemporaryTxt(dir string, formats []string, fileBase string) {
	if containsFormat(nativeOutputFormats(formats), config.OutputFormatTXT) {
		return
	}
	_ = os.Remove(filepath.Join(dir, fileBase+".txt"))
}

func appendCppOutputFlags(args []string, formats []string) []string {
	for _, f := range effectiveWhisperFormats(formats) {
		switch f {
		case config.OutputFormatTXT:
			args = append(args, "-otxt")
		case config.OutputFormatSRT:
			args = append(args, "-osrt")
		case config.OutputFormatVTT:
			args = append(args, "-ovtt")
		case config.OutputFormatJSON:
			args = append(args, "-ojson")
		}
	}
	return args
}

func listOutputFiles(dir string, cfg config.TranscriptionConfig, sessionDir string) []string {
	fileBase := outputFileBase(cfg, sessionDir)
	var out []string
	formats := config.NormalizeOutputFormats(cfg.OutputFormats)
	keep := keepNativeFormats(formats)
	for ext, format := range map[string]string{
		".txt":  config.OutputFormatTXT,
		".srt":  config.OutputFormatSRT,
		".vtt":  config.OutputFormatVTT,
		".json": config.OutputFormatJSON,
	} {
		if !keep[format] {
			continue
		}
		p := filepath.Join(dir, fileBase+ext)
		if _, err := os.Stat(p); err == nil {
			out = append(out, p)
		}
	}
	if config.WantsMarkdown(formats) {
		p := filepath.Join(dir, markdownFilename(cfg, sessionDir))
		if _, err := os.Stat(p); err == nil {
			out = append(out, p)
		}
	}
	return out
}
