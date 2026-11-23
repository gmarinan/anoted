package transcribe

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"anoted/internal/config"
)

// ErrDownloadInUse is returned when removing what the configured engine runs.
var ErrDownloadInUse = errors.New("in use by the configured transcription backend")

// Download is a runtime or model anoted installed, or one its backends
// fetched on first use. Together they are most of anoted's disk footprint:
// a CUDA PyTorch venv alone is several gigabytes.
type Download struct {
	// ID is stable across runs and short enough to type: `anoted models rm <id>`.
	ID    string
	Label string
	Path  string
	Bytes int64
	// InUse marks what the configured backend needs to transcribe; removing it
	// would break the next transcription, so RemoveDownload refuses.
	InUse bool
	// Shared marks the Hugging Face cache, which other programs also use.
	Shared bool

	// parent is the folder Path must sit directly in. RemoveDownload checks it
	// again right before deleting.
	parent string
	// extra are leftovers removed alongside Path (a Hugging Face lock folder).
	extra []string
}

// Contains reports whether path lies inside the download, e.g. whether a
// configured transcription.binary points into a venv about to be removed.
func (d Download) Contains(path string) bool {
	if path == "" || d.Path == "" {
		return false
	}
	return pathWithin(path, d.Path)
}

// Downloads lists what is on disk, largest first, with InUse set for cfg.
// It walks every folder to size it and may probe Python, so call it off the
// UI thread.
func Downloads(cfg config.TranscriptionConfig) []Download {
	return listDownloads(defaultDownloadRoots(), currentEngine(cfg))
}

// RemoveDownload deletes the download with id and returns what was removed.
//
// The path is looked up again here rather than taken from a caller's earlier
// listing, so a stale list can never point the deletion somewhere else.
func RemoveDownload(cfg config.TranscriptionConfig, id string) (Download, error) {
	return removeDownload(defaultDownloadRoots(), currentEngine(cfg), id)
}

type downloadRoots struct {
	data       string // anoted's data folder, holding its venvs and models
	legacyData string // the same folder from before the rename to anoted
	hub        string // Hugging Face hub cache (faster-whisper models)
	whisper    string // openai-whisper .pt and whisper.cpp ggml models
}

func defaultDownloadRoots() downloadRoots {
	return downloadRoots{
		data:       filepath.Dir(ManagedVenvDir()),
		legacyData: filepath.Dir(legacyManagedVenvDir()),
		hub:        huggingFaceHubDir(),
		whisper:    whisperCacheDir(),
	}
}

// activeEngine is what a transcription would run right now.
type activeEngine struct {
	binary    string
	backend   string
	model     string
	modelPath string
}

func currentEngine(cfg config.TranscriptionConfig) activeEngine {
	bin, backend, err := resolveBinary(cfg)
	if err != nil {
		// Nothing resolves, so nothing on disk is needed by it.
		return activeEngine{}
	}
	return activeEngine{binary: bin, backend: backend, model: resolvedModel(cfg), modelPath: cfg.ModelPath}
}

func listDownloads(roots downloadRoots, eng activeEngine) []Download {
	var out []Download

	addVenv := func(id, label, dir string) {
		if dir == "" {
			return
		}
		if _, err := os.Stat(filepath.Join(dir, "pyvenv.cfg")); err != nil {
			return
		}
		out = append(out, Download{
			ID: id, Label: label, Path: dir, parent: filepath.Dir(dir),
			InUse: eng.binary != "" && pathWithin(eng.binary, dir),
		})
	}
	addVenv("whisper-runtime", "Whisper runtime", filepath.Join(roots.data, managedVenvName))
	if roots.legacyData != roots.data {
		addVenv("whisper-runtime-meetctl", "Whisper runtime (old meetctl install)", filepath.Join(roots.legacyData, managedVenvName))
	}
	addVenv("parakeet-runtime", "Parakeet runtime", filepath.Join(roots.data, parakeetVenvName))

	if roots.data != "" {
		dir := filepath.Join(roots.data, "models", parakeetModelDir)
		if st, err := os.Stat(dir); err == nil && st.IsDir() {
			out = append(out, Download{
				ID: "parakeet-model", Label: "Parakeet TDT 0.6B v3 model", Path: dir, parent: filepath.Dir(dir),
				InUse: eng.backend == BackendParakeet,
			})
		}
	}

	out = append(out, hubDownloads(roots.hub, eng)...)
	out = append(out, whisperCacheDownloads(roots.whisper, eng)...)

	for i := range out {
		out[i].Bytes = diskUsage(out[i].Path)
	}
	sort.SliceStable(out, func(a, b int) bool { return out[a].Bytes > out[b].Bytes })
	return out
}

// hubDownloads lists the speech models in the Hugging Face cache. Only those:
// the cache is shared, and anything else in it belongs to other programs.
func hubDownloads(hub string, eng activeEngine) []Download {
	if hub == "" {
		return nil
	}
	entries, err := os.ReadDir(hub)
	if err != nil {
		return nil
	}
	want := strings.ToLower(fasterWhisperRepo(eng.model))
	seen := map[string]bool{}
	var out []Download
	for _, e := range entries {
		owner, name, ok := hubRepo(e)
		if !ok {
			continue
		}
		lower := strings.ToLower(name)
		var d Download
		switch {
		case strings.Contains(lower, "whisper") && (strings.Contains(lower, "faster") || strings.HasSuffix(lower, "-ct2")):
			d.Label = "faster-whisper " + strings.TrimPrefix(strings.TrimPrefix(lower, "faster-whisper-"), "faster-")
			d.InUse = eng.backend == BackendFasterWhisper && strings.ToLower(owner+"/"+name) == want
		case strings.EqualFold(owner, "istupakov") && strings.HasPrefix(lower, "parakeet"):
			// The parakeet runner downloads into its own folder; a copy here is
			// left over from an earlier version and nothing reads it.
			d.Label = "Parakeet model (unused cache copy)"
		default:
			continue
		}
		d.ID = lower
		if seen[d.ID] {
			d.ID = strings.ToLower(owner) + "-" + lower
		}
		seen[d.ID] = true
		d.Path = filepath.Join(hub, e.Name())
		d.parent = hub
		d.Shared = true
		d.extra = []string{filepath.Join(hub, ".locks", e.Name())}
		out = append(out, d)
	}
	return out
}

// hubRepo parses a hub cache folder name, models--<owner>--<name>.
func hubRepo(e fs.DirEntry) (owner, name string, ok bool) {
	if !e.IsDir() {
		return "", "", false
	}
	rest, found := strings.CutPrefix(e.Name(), "models--")
	if !found {
		return "", "", false
	}
	owner, name, ok = strings.Cut(rest, "--")
	return owner, name, ok && owner != "" && name != ""
}

// fasterWhisperRepo maps a model setting to the repository faster-whisper
// downloads it from, mirroring faster_whisper.utils._MODELS.
func fasterWhisperRepo(model string) string {
	switch {
	case model == "":
		return ""
	case strings.Contains(model, "/"):
		return model // already a repository id (or a local path, which matches nothing)
	case model == "large":
		return "Systran/faster-whisper-large-v3"
	case model == "turbo" || model == "large-v3-turbo":
		return "mobiuslabs/faster-whisper-large-v3-turbo"
	case model == "distil-large-v3.5":
		return "distil-whisper/distil-large-v3.5-ct2"
	case strings.HasPrefix(model, "distil-"):
		return "Systran/faster-distil-whisper-" + strings.TrimPrefix(model, "distil-")
	default:
		return "Systran/faster-whisper-" + model
	}
}

// whisperCacheDownloads lists openai-whisper checkpoints and whisper.cpp
// models, which share ~/.cache/whisper.
func whisperCacheDownloads(dir string, eng activeEngine) []Download {
	if dir == "" {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []Download
	for _, e := range entries {
		if !e.Type().IsRegular() {
			continue
		}
		path := filepath.Join(dir, e.Name())
		d := Download{Path: path, parent: dir}
		switch {
		case strings.HasPrefix(e.Name(), "ggml-") && strings.HasSuffix(e.Name(), ".bin"):
			stem := strings.TrimSuffix(strings.TrimPrefix(e.Name(), "ggml-"), ".bin")
			d.ID, d.Label = "whisper-cpp-"+stem, "whisper.cpp "+stem
			if eng.backend == BackendWhisperCpp {
				if eng.modelPath != "" {
					d.InUse = pathWithin(eng.modelPath, path)
				} else {
					d.InUse = stem == eng.model
				}
			}
		case strings.HasSuffix(e.Name(), ".pt"):
			stem := strings.TrimSuffix(e.Name(), ".pt")
			d.ID, d.Label = "openai-whisper-"+stem, "openai-whisper "+stem
			d.InUse = eng.backend == BackendOpenAI && stem == openaiWhisperCheckpoint(eng.model)
		default:
			continue
		}
		out = append(out, d)
	}
	return out
}

// openaiWhisperCheckpoint is the file name openai-whisper stores a model under.
func openaiWhisperCheckpoint(model string) string {
	switch model {
	case "large":
		return "large-v3"
	case "turbo":
		return "large-v3-turbo"
	default:
		return model
	}
}

func removeDownload(roots downloadRoots, eng activeEngine, id string) (Download, error) {
	id = strings.ToLower(strings.TrimSpace(id))
	var d Download
	found := false
	for _, it := range listDownloads(roots, eng) {
		if it.ID == id {
			d, found = it, true
			break
		}
	}
	if !found {
		return Download{}, fmt.Errorf("no download named %q", id)
	}
	if d.InUse {
		return d, fmt.Errorf("%s: %w — switch transcription.backend first", d.Label, ErrDownloadInUse)
	}
	if err := safeRemove(d.Path, d.parent); err != nil {
		return d, err
	}
	for _, p := range d.extra {
		_ = safeRemove(p, filepath.Dir(p))
	}
	if d.ID == "parakeet-model" {
		// Leave no empty models/ folder behind; Remove fails harmlessly if
		// anything else is still in it.
		_ = os.Remove(d.parent)
	}
	// The import and CUDA probes cache answers about interpreters that may
	// just have been deleted.
	invalidateImportProbes()
	InvalidateTorchCUDACache()
	return d, nil
}

// safeRemove deletes path, which must sit directly inside parent.
//
// The paths come from fixed locations and a directory listing, so this should
// never fire; it is here because the alternative failure is RemoveAll on a
// home directory.
func safeRemove(path, parent string) error {
	if path == "" || parent == "" {
		return errors.New("refusing to remove: empty path")
	}
	path, parent = filepath.Clean(path), filepath.Clean(parent)
	if !filepath.IsAbs(path) || filepath.Dir(path) != parent {
		return fmt.Errorf("refusing to remove %s: not inside %s", path, parent)
	}
	if home, err := os.UserHomeDir(); err == nil && filepath.Clean(home) == path {
		return fmt.Errorf("refusing to remove %s: home folder", path)
	}
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("remove %s: %w", path, err)
	}
	return nil
}

// diskUsage totals the regular files under path. Symlinks are not followed:
// the Hugging Face cache links each snapshot file to a blob in the same
// folder, which is already counted.
func diskUsage(path string) int64 {
	var total int64
	_ = filepath.WalkDir(path, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // an unreadable entry just goes uncounted
		}
		if d.Type().IsRegular() {
			if info, err := d.Info(); err == nil {
				total += info.Size()
			}
		}
		return nil
	})
	return total
}

func pathWithin(path, dir string) bool {
	absPath, err1 := filepath.Abs(path)
	absDir, err2 := filepath.Abs(dir)
	if err1 != nil || err2 != nil {
		return false
	}
	rel, err := filepath.Rel(absDir, absPath)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

// huggingFaceHubDir follows huggingface_hub's own lookup order.
func huggingFaceHubDir() string {
	for _, env := range []string{"HF_HUB_CACHE", "HUGGINGFACE_HUB_CACHE"} {
		if v := os.Getenv(env); v != "" {
			return v
		}
	}
	if v := os.Getenv("HF_HOME"); v != "" {
		return filepath.Join(v, "hub")
	}
	if base := userCacheBase(); base != "" {
		return filepath.Join(base, "huggingface", "hub")
	}
	return ""
}

// whisperCacheDir is where openai-whisper downloads checkpoints; anoted looks
// for whisper.cpp models there too.
func whisperCacheDir() string {
	if base := userCacheBase(); base != "" {
		return filepath.Join(base, "whisper")
	}
	return ""
}

// userCacheBase is ~/.cache, honouring XDG_CACHE_HOME the way the Python
// libraries do. They use ~/.cache on Windows as well, not %LOCALAPPDATA%.
func userCacheBase() string {
	if v := os.Getenv("XDG_CACHE_HOME"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".cache")
}
