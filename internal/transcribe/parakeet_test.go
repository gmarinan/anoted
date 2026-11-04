package transcribe

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"anoted/internal/config"
)

func isolateDataDir(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("XDG_DATA_HOME", root)
	t.Setenv("LOCALAPPDATA", root)
	invalidateImportProbes()
	t.Cleanup(invalidateImportProbes)
	return root
}

func TestParakeetPathsSitNextToTheWhisperVenv(t *testing.T) {
	root := isolateDataDir(t)
	if got, want := ParakeetVenvDir(), filepath.Join(root, "anoted", "parakeet-venv"); got != want {
		t.Fatalf("venv = %q, want %q", got, want)
	}
	// A separate venv is the point: faster-whisper's CPU onnxruntime and
	// parakeet's onnxruntime-gpu install the same module.
	if ParakeetVenvDir() == ManagedVenvDir() {
		t.Fatal("parakeet must not share the Whisper venv")
	}
	if got, want := ParakeetModelDir(), filepath.Join(root, "anoted", "models", "parakeet-tdt-0.6b-v3-onnx"); got != want {
		t.Fatalf("model dir = %q, want %q", got, want)
	}
}

func TestResolveParakeetNotInstalledPointsAtInstall(t *testing.T) {
	isolateDataDir(t)
	_, _, err := resolveBinary(config.TranscriptionConfig{Backend: "parakeet"})
	if err == nil || !strings.Contains(err.Error(), "anoted parakeet install") {
		t.Fatalf("err = %v, want a pointer to `anoted parakeet install`", err)
	}
}

func TestResolveParakeetIgnoresConfiguredWhisperBinary(t *testing.T) {
	// binary: points at the openai-whisper CLI in real configs; it must not
	// hijack a parakeet selection the way it would for auto.
	isolateDataDir(t)
	_, backend, err := resolveBinary(config.TranscriptionConfig{Backend: "Parakeet", Binary: os.Args[0]})
	if err == nil {
		t.Fatalf("resolved to %q; want the not-installed error", backend)
	}
}

func TestParakeetModelIsNotTheWhisperModel(t *testing.T) {
	cfg := config.TranscriptionConfig{Backend: "parakeet", Model: "large"}
	if got := resolvedModel(cfg); got != ParakeetModel {
		t.Fatalf("model = %q, want %q", got, ParakeetModel)
	}
	full, light := modelVRAMBudgetMiB(resolvedModel(cfg))
	if full != light || full < 5*1024 {
		t.Fatalf("budget = %d/%d MiB, want one bar of at least the measured ~5 GB", full, light)
	}
	if got := resolvedModel(config.TranscriptionConfig{Backend: "faster-whisper", Model: "large"}); got != "large" {
		t.Fatalf("whisper model = %q, want large", got)
	}
}

func TestParakeetDeviceAutoFollowsTheDriver(t *testing.T) {
	got := resolveDevice(config.TranscriptionConfig{Backend: "parakeet", Device: "auto"})
	want := DeviceCPU
	if NVIDIAAvailable() {
		want = DeviceCUDA
	}
	if got != want {
		t.Fatalf("device = %q, want %q", got, want)
	}
	if got := resolveDevice(config.TranscriptionConfig{Backend: "parakeet", Device: "cpu"}); got != DeviceCPU {
		t.Fatalf("explicit cpu = %q", got)
	}
}

func TestParakeetArgs(t *testing.T) {
	isolateDataDir(t)
	args := parakeetArgs(config.TranscriptionConfig{Language: " es "}, "/tmp/a.wav", DeviceCUDA)
	if args[0] != "-" {
		t.Fatalf("runner must be read from stdin, args = %v", args)
	}
	for _, pair := range [][2]string{
		{"--audio", "/tmp/a.wav"},
		{"--model-dir", ParakeetModelDir()},
		{"--device", "cuda"},
		{"--language", "es"},
	} {
		i := slices.Index(args, pair[0])
		if i < 0 || i+1 >= len(args) || args[i+1] != pair[1] {
			t.Fatalf("missing %s %s in %v", pair[0], pair[1], args)
		}
	}
	if slices.Contains(parakeetArgs(config.TranscriptionConfig{}, "a.wav", DeviceCPU), "--language") {
		t.Fatal("auto language must not pass --language")
	}
}

func TestParakeetPackages(t *testing.T) {
	gpu, _ := parakeetPackages(true)
	if !slices.Contains(gpu, "onnxruntime-gpu[cuda,cudnn]") {
		t.Fatalf("GPU install must bring CUDA and cuDNN wheels: %v", gpu)
	}
	cpu, _ := parakeetPackages(false)
	if slices.ContainsFunc(cpu, func(p string) bool { return strings.Contains(p, "gpu") }) {
		t.Fatalf("CPU install must not pull onnxruntime-gpu: %v", cpu)
	}
}

func TestParakeetModelFilesMatchRunner(t *testing.T) {
	block := parakeetRunner[strings.Index(parakeetRunner, "MODEL_FILES = ["):]
	block = block[:strings.Index(block, "]")]
	for _, f := range parakeetModelFiles {
		if !strings.Contains(block, `"`+f+`"`) {
			t.Fatalf("%s is checked by Go but not downloaded by the runner", f)
		}
	}
	if got := strings.Count(block, `"`); got != 2*len(parakeetModelFiles) {
		t.Fatalf("runner downloads %d files, Go checks %d", got/2, len(parakeetModelFiles))
	}
}

func TestParakeetModelDownloadedNeedsEveryFile(t *testing.T) {
	isolateDataDir(t)
	dir := ParakeetModelDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for i, f := range parakeetModelFiles {
		if ParakeetModelDownloaded() {
			t.Fatalf("reported downloaded with only %d of %d files", i, len(parakeetModelFiles))
		}
		if err := os.WriteFile(filepath.Join(dir, f), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if !ParakeetModelDownloaded() {
		t.Fatal("all files present but not reported downloaded")
	}
}

// The English-drift guard decides what ends up in the transcript, so its pure
// helpers are exercised with a real interpreter. numpy and onnx-asr are not
// needed for these; the test skips when there is no Python at all.
const runnerLogicCheck = `
import importlib.util, sys
spec = importlib.util.spec_from_file_location("runner", sys.argv[1])
r = importlib.util.module_from_spec(spec); spec.loader.exec_module(r)

w = r.words_from_tokens([" Bu", "eno", ",", " ¿", "C", "ómo"], [0.0, 0.08, 0.16, 0.5, 0.58, 0.66], 10.0)
assert [x["text"] for x in w] == ["Bueno,", "¿Cómo"], w
assert w[0]["start"] == 10.0 and abs(w[0]["end"] - 10.24) < 1e-9, w

def words(text, start=0.0, step=0.3):
    return [{"start": start + i * step, "end": start + i * step + 0.2, "text": t} for i, t in enumerate(text.split())]

segs = r.segments_from_words(words("Hola a todos. ¿Cómo están?") + words("Bien", start=10))
assert [s["text"] for s in segs] == [" Hola a todos.", " ¿Cómo están?", " Bien"], segs

es = words("vamos a revisar el deploy del ticket con el equipo de QA mañana")
en = words("we are going to have the transfer of the administration and that is it")
assert r.english_words(es) == 0, "a Spanish sentence with English nouns is not drift"
assert r.english_words(en) == len(en), r.english_words(en)

condensed = words("we are going to have the transfer")
recovered = words("lo que les comenté en relación al tema de que vamos a tener capacidades adicionales dentro de Google")
assert r.least_english([condensed, recovered]) is recovered, "the Spanish re-decode must win"
assert r.least_english([recovered, words("hola")]) is recovered, "dropping speech is never a fix"
assert r.least_english([en, words("we have the transfer of the administration and that is it ok")])[0]["text"] == "we", "real English stays"
print("ok")
`

func TestParakeetRunnerLogic(t *testing.T) {
	py := ""
	for _, name := range []string{"python3", "python"} {
		if p, err := exec.LookPath(name); err == nil {
			py = p
			break
		}
	}
	if py == "" {
		t.Skip("no python interpreter")
	}
	dir := t.TempDir()
	runner := filepath.Join(dir, "parakeet_runner.py")
	if err := os.WriteFile(runner, []byte(parakeetRunner), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(py, "-I", "-c", runnerLogicCheck, runner).CombinedOutput()
	if err != nil {
		// Windows' python.exe can be the Microsoft Store alias, which only
		// prints how to install Python.
		if strings.Contains(string(out), "Microsoft Store") {
			t.Skip("python is the Microsoft Store alias")
		}
		t.Fatalf("runner logic: %v\n%s", err, out)
	}
	if strings.TrimSpace(string(out)) != "ok" {
		t.Fatalf("unexpected output: %s", out)
	}
}
