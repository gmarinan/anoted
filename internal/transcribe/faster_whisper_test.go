package transcribe

import (
	"strings"
	"testing"
)

const runnerOutput = `{"type": "info", "duration": 100.0, "language": "es"}
{"type": "segment", "start": 0.0, "end": 25.0, "text": " Hola"}
not json at all — diagnostic noise on stdout
{"type": "segment", "start": 25.0, "end": 100.0, "text": " Adiós"}
`

func TestParseRunnerStreamProgressIsExact(t *testing.T) {
	// The runner reports duration up front, so progress is computed rather than
	// guessed — this is what the openai-whisper path cannot do, since it only
	// sees segment timestamps and skips silence.
	var got []float64
	segs, lang, runnerErr, err := parseRunnerStream(strings.NewReader(runnerOutput),
		func(p Progress) {
			if p.SegmentText != "" {
				got = append(got, p.Percent)
			}
		})
	if err != nil {
		t.Fatalf("parseRunnerStream: %v", err)
	}
	if runnerErr != "" {
		t.Fatalf("unexpected runner error: %s", runnerErr)
	}
	if lang != "es" {
		t.Fatalf("language = %q, want es", lang)
	}
	if len(segs) != 2 {
		t.Fatalf("segments = %d, want 2 (non-JSON line must be ignored)", len(segs))
	}
	if len(got) != 2 || got[0] != 25 || got[1] != 100 {
		t.Fatalf("progress = %v, want [25 100]", got)
	}
}

func TestParseRunnerStreamSurfacesRunnerError(t *testing.T) {
	// A model that fails to load reports through the protocol, not the exit
	// code alone; losing that message would leave the user with "exit status 1".
	_, _, runnerErr, err := parseRunnerStream(
		strings.NewReader(`{"type":"error","message":"load model: out of memory"}`+"\n"), nil)
	if err != nil {
		t.Fatalf("parseRunnerStream: %v", err)
	}
	if !strings.Contains(runnerErr, "out of memory") {
		t.Fatalf("runnerErr = %q, want the model's message", runnerErr)
	}
}

func TestParseRunnerStreamClampsPastDuration(t *testing.T) {
	// VAD can place a final segment slightly past the reported duration; the
	// bar must not render above 100%.
	var last float64
	_, _, _, err := parseRunnerStream(strings.NewReader(
		`{"type":"info","duration":10.0}`+"\n"+
			`{"type":"segment","start":9.0,"end":11.5,"text":"fin"}`+"\n"),
		func(p Progress) {
			if p.SegmentText != "" {
				last = p.Percent
			}
		})
	if err != nil {
		t.Fatalf("parseRunnerStream: %v", err)
	}
	if last != 100 {
		t.Fatalf("percent = %v, want clamped to 100", last)
	}
}

func TestVenvDirForBinary(t *testing.T) {
	if got := venvDirForBinary("/home/u/.local/share/anoted/whisper-venv/bin/whisper"); got != "/home/u/.local/share/anoted/whisper-venv" {
		t.Fatalf("venvDirForBinary = %q", got)
	}
	if got := venvDirForBinary("whisper"); got != "" {
		t.Fatalf("bare name should not resolve to a venv, got %q", got)
	}
}

func TestFasterWhisperComputeType(t *testing.T) {
	// float16 on GPU is the same arithmetic openai-whisper uses, so switching
	// backends does not change the numerics; int8 is only for CPU.
	if got := fasterWhisperComputeType(DeviceCUDA); got != "float16" {
		t.Fatalf("cuda compute type = %q, want float16", got)
	}
	if got := fasterWhisperComputeType(DeviceCPU); got != "int8" {
		t.Fatalf("cpu compute type = %q, want int8", got)
	}
}
