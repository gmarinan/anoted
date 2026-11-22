package filejob

import (
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ParseDropped turns the text a terminal delivers when files are dragged onto
// it into the paths it names.
//
// There is no one format. Alacritty pastes the bare path followed by a space,
// spaces inside it unescaped; GNOME Terminal and Kitty wrap it in single
// quotes; Windows Terminal uses double quotes when the path has spaces; some
// terminals backslash-escape; file managers on the far side of the drop may
// hand over file:// URIs. Several files arrive on one line or one per line.
//
// isFile settles the ambiguity that leaves: a line is tried whole first, so a
// bare path with spaces is one file rather than several fragments, and only if
// that names nothing is it split as a shell would split it. Paths that do not
// exist are still returned, so the caller can say which one was not found.
func ParseDropped(text string, isFile func(string) bool) []string {
	var out []string
	seen := make(map[string]bool)
	add := func(p string) {
		if p != "" && !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	lines := strings.FieldsFunc(text, func(r rune) bool { return r == '\n' || r == '\r' || r == 0 })
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if whole := cleanPath(line); isFile(whole) {
			add(whole)
			continue
		}
		for _, tok := range splitShellWords(line) {
			add(cleanPath(tok))
		}
	}
	return out
}

// IsFile reports whether path names an existing regular file.
func IsFile(path string) bool {
	if path == "" {
		return false
	}
	st, err := os.Stat(path)
	return err == nil && st.Mode().IsRegular()
}

// cleanPath strips the wrapping a terminal put around a path.
func cleanPath(tok string) string {
	tok = strings.TrimSpace(tok)
	if len(tok) >= 2 {
		if q := tok[0]; (q == '\'' || q == '"') && tok[len(tok)-1] == q {
			tok = tok[1 : len(tok)-1]
		}
	}
	if strings.HasPrefix(strings.ToLower(tok), "file://") {
		if u, err := url.Parse(tok); err == nil && u.Path != "" {
			tok = u.Path
			// file:///C:/Users/x parses to /C:/Users/x.
			if len(tok) >= 3 && tok[0] == '/' && tok[2] == ':' {
				tok = tok[1:]
			}
		}
	}
	if tok == "" {
		return ""
	}
	if strings.HasPrefix(tok, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			tok = filepath.Join(home, tok[2:])
		}
	}
	return filepath.Clean(filepath.FromSlash(tok))
}

// splitShellWords splits on unquoted whitespace, honouring single quotes,
// double quotes and — except on Windows, where it is the path separator —
// backslash escapes.
func splitShellWords(s string) []string {
	escapes := runtime.GOOS != "windows"
	var (
		words []string
		cur   strings.Builder
		quote rune
		open  bool
	)
	flush := func() {
		if open {
			words = append(words, cur.String())
			cur.Reset()
			open = false
		}
	}
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else if r == '\\' && quote == '"' && escapes && i+1 < len(runes) {
				i++
				cur.WriteRune(runes[i])
			} else {
				cur.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote = r
			open = true
		case r == '\\' && escapes && i+1 < len(runes):
			i++
			cur.WriteRune(runes[i])
			open = true
		case r == ' ' || r == '\t':
			flush()
		default:
			cur.WriteRune(r)
			open = true
		}
	}
	flush()
	return words
}

// mediaExtensions are the containers ffmpeg is routinely asked to open here.
var mediaExtensions = map[string]bool{
	".wav": true, ".mp3": true, ".m4a": true, ".aac": true, ".ogg": true, ".oga": true,
	".opus": true, ".flac": true, ".wma": true, ".amr": true, ".aiff": true, ".aif": true,
	".caf": true, ".3gp": true, ".mka": true, ".weba": true,
	".mp4": true, ".m4v": true, ".mkv": true, ".webm": true, ".mov": true, ".avi": true,
	".wmv": true, ".flv": true, ".ts": true, ".mpg": true, ".mpeg": true, ".ogv": true,
}

// LooksLikeMedia reports whether the file name carries an audio or video
// extension. It is a guard against treating arbitrary pasted text as a file to
// transcribe, not a statement of what ffmpeg can decode.
func LooksLikeMedia(path string) bool {
	return mediaExtensions[strings.ToLower(filepath.Ext(path))]
}
