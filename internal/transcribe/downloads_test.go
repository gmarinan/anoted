package transcribe

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, make([]byte, size), 0o600); err != nil {
		t.Fatal(err)
	}
}

// fakeRoots lays out a machine with every kind of download plus things that
// are not anoted's and must never be listed.
func fakeRoots(t *testing.T) downloadRoots {
	t.Helper()
	base := t.TempDir()
	r := downloadRoots{
		data:       filepath.Join(base, "share", "anoted"),
		legacyData: filepath.Join(base, "share", "meetctl"),
		hub:        filepath.Join(base, "cache", "huggingface", "hub"),
		whisper:    filepath.Join(base, "cache", "whisper"),
	}
	writeFile(t, filepath.Join(r.legacyData, "whisper-venv", "pyvenv.cfg"), 10)
	writeFile(t, filepath.Join(r.legacyData, "whisper-venv", "bin", "whisper"), 500)
	writeFile(t, filepath.Join(r.data, "parakeet-venv", "pyvenv.cfg"), 10)
	writeFile(t, filepath.Join(r.data, "parakeet-venv", "bin", "python"), 300)
	writeFile(t, filepath.Join(r.data, "models", parakeetModelDir, "encoder-model.onnx"), 400)
	writeFile(t, filepath.Join(r.data, "files", "job", "job.json"), 5) // transcripts, not a download

	fw := filepath.Join(r.hub, "models--Systran--faster-whisper-large-v3")
	writeFile(t, filepath.Join(fw, "blobs", "abc"), 200)
	if err := os.MkdirAll(filepath.Join(fw, "snapshots", "rev"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Snapshot files are symlinks to blobs; counting them would double the size.
	if err := os.Symlink(filepath.Join("..", "..", "blobs", "abc"), filepath.Join(fw, "snapshots", "rev", "model.bin")); err != nil {
		t.Skip("symlinks unavailable:", err)
	}
	writeFile(t, filepath.Join(r.hub, ".locks", "models--Systran--faster-whisper-large-v3", "x.lock"), 0)
	writeFile(t, filepath.Join(r.hub, "models--mobiuslabs--faster-whisper-large-v3-turbo", "blobs", "t"), 50)
	writeFile(t, filepath.Join(r.hub, "models--black-forest-labs--FLUX.2-klein-4B", "blobs", "f"), 999)

	writeFile(t, filepath.Join(r.whisper, "large-v3-turbo.pt"), 120)
	writeFile(t, filepath.Join(r.whisper, "ggml-base.bin"), 80)
	writeFile(t, filepath.Join(r.whisper, "notes.txt"), 1)
	return r
}

func byID(ds []Download) map[string]Download {
	m := map[string]Download{}
	for _, d := range ds {
		m[d.ID] = d
	}
	return m
}

func TestListDownloadsFindsOnlySpeechModels(t *testing.T) {
	r := fakeRoots(t)
	got := byID(listDownloads(r, activeEngine{}))

	want := map[string]int64{
		"whisper-runtime-meetctl":       510,
		"parakeet-runtime":              310,
		"parakeet-model":                400,
		"faster-whisper-large-v3":       200,
		"faster-whisper-large-v3-turbo": 50,
		"openai-whisper-large-v3-turbo": 120,
		"whisper-cpp-base":              80,
	}
	if len(got) != len(want) {
		t.Fatalf("got %d downloads: %+v", len(got), got)
	}
	for id, size := range want {
		d, ok := got[id]
		if !ok {
			t.Errorf("%s missing", id)
			continue
		}
		if d.Bytes != size {
			t.Errorf("%s: %d bytes, want %d", id, d.Bytes, size)
		}
		if d.InUse {
			t.Errorf("%s in use with no engine configured", id)
		}
	}
	if !got["faster-whisper-large-v3"].Shared || got["parakeet-model"].Shared {
		t.Error("Shared marks the Hugging Face cache only")
	}
}

func TestListDownloadsLargestFirst(t *testing.T) {
	ds := listDownloads(fakeRoots(t), activeEngine{})
	for i := 1; i < len(ds); i++ {
		if ds[i].Bytes > ds[i-1].Bytes {
			t.Fatalf("not sorted: %s (%d) after %s (%d)", ds[i].ID, ds[i].Bytes, ds[i-1].ID, ds[i-1].Bytes)
		}
	}
}

func TestInUseFollowsTheConfiguredEngine(t *testing.T) {
	r := fakeRoots(t)
	cases := []struct {
		name string
		eng  activeEngine
		want []string
	}{
		{"parakeet", activeEngine{
			binary: filepath.Join(r.data, "parakeet-venv", "bin", "python"), backend: BackendParakeet, model: ParakeetModel,
		}, []string{"parakeet-runtime", "parakeet-model"}},
		{"faster-whisper turbo", activeEngine{
			binary: filepath.Join(r.legacyData, "whisper-venv", "bin", "python"), backend: BackendFasterWhisper, model: "turbo",
		}, []string{"whisper-runtime-meetctl", "faster-whisper-large-v3-turbo"}},
		{"openai-whisper turbo", activeEngine{
			binary: filepath.Join(r.legacyData, "whisper-venv", "bin", "whisper"), backend: BackendOpenAI, model: "turbo",
		}, []string{"whisper-runtime-meetctl", "openai-whisper-large-v3-turbo"}},
		{"whisper.cpp from PATH", activeEngine{
			binary: "/usr/bin/whisper-cli", backend: BackendWhisperCpp, model: "base",
		}, []string{"whisper-cpp-base"}},
	}
	for _, tc := range cases {
		var inUse []string
		for _, d := range listDownloads(r, tc.eng) {
			if d.InUse {
				inUse = append(inUse, d.ID)
			}
		}
		if strings.Join(sorted(inUse), ",") != strings.Join(sorted(tc.want), ",") {
			t.Errorf("%s: in use %v, want %v", tc.name, inUse, tc.want)
		}
	}
}

func sorted(s []string) []string {
	out := append([]string(nil), s...)
	for i := range out {
		for j := i + 1; j < len(out); j++ {
			if out[j] < out[i] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

func TestRemoveDownload(t *testing.T) {
	r := fakeRoots(t)
	eng := activeEngine{binary: filepath.Join(r.data, "parakeet-venv", "bin", "python"), backend: BackendParakeet}

	// What the engine runs is refused, and stays on disk.
	if _, err := removeDownload(r, eng, "parakeet-model"); !errors.Is(err, ErrDownloadInUse) {
		t.Fatalf("removing the model in use: %v", err)
	}
	if _, err := os.Stat(filepath.Join(r.data, "models", parakeetModelDir)); err != nil {
		t.Fatal("a refused remove deleted the model")
	}

	d, err := removeDownload(r, eng, "whisper-runtime-meetctl")
	if err != nil {
		t.Fatal(err)
	}
	if d.Bytes != 510 || !d.Contains(filepath.Join(r.legacyData, "whisper-venv", "bin", "whisper")) {
		t.Fatalf("removed %+v", d)
	}
	if _, err := os.Stat(d.Path); !os.IsNotExist(err) {
		t.Fatal("venv still on disk")
	}

	// A Hugging Face model goes with its lock folder, and nothing else in the
	// shared cache is touched.
	if _, err := removeDownload(r, eng, "FASTER-WHISPER-LARGE-V3"); err != nil {
		t.Fatal(err)
	}
	for _, gone := range []string{"models--Systran--faster-whisper-large-v3", filepath.Join(".locks", "models--Systran--faster-whisper-large-v3")} {
		if _, err := os.Stat(filepath.Join(r.hub, gone)); !os.IsNotExist(err) {
			t.Errorf("%s still on disk", gone)
		}
	}
	if _, err := os.Stat(filepath.Join(r.hub, "models--black-forest-labs--FLUX.2-klein-4B")); err != nil {
		t.Fatal("another program's model was deleted")
	}

	if _, err := removeDownload(r, eng, "whisper-cpp-base"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(r.whisper, "notes.txt")); err != nil {
		t.Fatal("a file next to the model was deleted")
	}

	if _, err := removeDownload(r, eng, "nope"); err == nil {
		t.Fatal("removing an unknown id succeeded")
	}
}

func TestSafeRemoveRefusesOutsideParent(t *testing.T) {
	root := t.TempDir()
	keep := filepath.Join(root, "a", "keep.txt")
	writeFile(t, keep, 1)
	for _, tc := range [][2]string{
		{"", root},
		{filepath.Join(root, "a"), ""},
		{root, root},
		{filepath.Join(root, "a"), filepath.Join(root, "b")},
		{filepath.Join(root, "a", "..", "..", "x"), root},
		{"relative", "."},
	} {
		if err := safeRemove(tc[0], tc[1]); err == nil {
			t.Errorf("safeRemove(%q, %q) succeeded", tc[0], tc[1])
		}
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatal("a refused remove deleted files")
	}
}

func TestFasterWhisperRepo(t *testing.T) {
	for model, want := range map[string]string{
		"large":           "Systran/faster-whisper-large-v3",
		"large-v2":        "Systran/faster-whisper-large-v2",
		"turbo":           "mobiuslabs/faster-whisper-large-v3-turbo",
		"distil-large-v3": "Systran/faster-distil-whisper-large-v3",
		"me/custom-ct2":   "me/custom-ct2",
		"":                "",
	} {
		if got := fasterWhisperRepo(model); got != want {
			t.Errorf("fasterWhisperRepo(%q) = %q, want %q", model, got, want)
		}
	}
}
