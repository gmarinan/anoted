"""Run NVIDIA Parakeet TDT 0.6B v3 (via onnx-asr) and stream results as JSON lines.

Same contract as faster_whisper_runner.py, so the Go side parses both alike:

    {"type": "status",  "stage": "loading model"}
    {"type": "info",    "duration": 1234.5, "language": ""}
    {"type": "segment", "start": 0.0, "end": 5.0, "text": " Hola"}
    {"type": "error",   "message": "..."}

Why onnx-asr and not NeMo: same weights and, on real meetings, the same
output (0.6 % word difference, the size of NeMo's own fp32/bf16 noise), with a
fraction of the install (no PyTorch), a model load of ~2 s instead of ~20 s,
and it runs on the interpreter anoted already finds, Windows included.

Long audio is split, never fed whole: full attention over a meeting is
quadratic and does not fit a consumer GPU (25 minutes already needs >12 GB).
Cuts land on the quietest half second near every CHUNK_S seconds so no word
is sliced in two.

Parakeet has no language parameter: it decides per chunk, and on Spanish
meetings it now and then decides English and *translates* the chunk, also
condensing it (94 English words where the speech had 200). Which chunk drifts
depends on where it was cut, not on the audio, and the model is fast enough
to decode a stretch several times, so a chunk that comes out in English is
re-decoded with different cuts and the least-English version is kept. On four
real meetings that took English from 2.4 % of the words to 0.8 % and brought
back ~600 words of Spanish. Every candidate is the same audio, so a passage
that really is English stays English; with --language en, or on auto once most
of the meeting has come out English, the check is off.
"""

import argparse
import json
import os
import shutil
import subprocess
import sys
import wave

MODEL_NAME = "nemo-parakeet-tdt-0.6b-v3"
MODEL_REPO = "istupakov/parakeet-tdt-0.6b-v3-onnx"
MODEL_FILES = [
    "config.json",
    "vocab.txt",
    "encoder-model.onnx",
    "encoder-model.onnx.data",
    "decoder_joint-model.onnx",
]
CHUNK_S = 120.0
SLACK_S = 20.0
# Encoder frame: 10 ms feature hop x 8 subsampling. Token timestamps mark where
# a token starts, so a word's end is its last token plus one frame.
FRAME_S = 0.08
# A segment is a line in the transcript. Close one at sentence punctuation, at
# a pause, or before it grows into a paragraph nobody can scan.
SENTENCE_END = (".", "?", "!", "…")
PAUSE_S = 1.0
MAX_SEGMENT_S = 30.0


def emit(obj):
    json.dump(obj, sys.stdout, ensure_ascii=False)
    sys.stdout.write("\n")
    sys.stdout.flush()


def model_present(model_dir):
    return all(os.path.isfile(os.path.join(model_dir, f)) for f in MODEL_FILES)


def ensure_model(model_dir):
    """Download the ONNX export into model_dir as plain files.

    Not the Hugging Face cache: it stores weights as symlinks into a blobs
    directory, and onnxruntime >= 1.31 refuses external weights whose real path
    leaves the model directory.
    """
    if model_present(model_dir):
        return
    emit({"type": "status", "stage": "downloading Parakeet model (~2.4 GB, first run only)"})
    from huggingface_hub import snapshot_download

    snapshot_download(MODEL_REPO, local_dir=model_dir, allow_patterns=MODEL_FILES)
    if not model_present(model_dir):
        raise RuntimeError(f"model download incomplete in {model_dir}")


def load_audio(path, np):
    """Mono int16 samples plus their rate.

    anoted writes 16-bit PCM WAV on every platform, which the standard library
    reads without ffmpeg (often absent on Windows). Anything else falls back to
    ffmpeg. Samples stay int16: a long meeting at 48 kHz is half the memory of
    float32, and each chunk is converted only when it is recognised.
    """
    try:
        with wave.open(path, "rb") as w:
            if w.getsampwidth() != 2:
                raise wave.Error("not 16-bit PCM")
            channels, rate, frames = w.getnchannels(), w.getframerate(), w.getnframes()
            out = np.empty(frames, dtype=np.int16)
            pos, block = 0, rate * 60
            while pos < frames:
                raw = w.readframes(block)
                if not raw:
                    break
                pcm = np.frombuffer(raw, dtype="<i2").reshape(-1, channels)
                n = len(pcm)
                out[pos:pos + n] = pcm.mean(axis=1, dtype=np.float32).astype(np.int16)
                pos += n
            return out[:pos], rate
    except (wave.Error, EOFError):
        pass
    ffmpeg = shutil.which("ffmpeg")
    if not ffmpeg:
        raise RuntimeError(f"{path} is not 16-bit PCM WAV and ffmpeg is not installed to convert it")
    raw = subprocess.run(
        [ffmpeg, "-v", "error", "-i", path, "-ac", "1", "-ar", "16000", "-f", "s16le", "-"],
        check=True, capture_output=True,
    ).stdout
    return np.frombuffer(raw, dtype="<i2"), 16000


def split(x, rate, np, target=CHUNK_S, slack=SLACK_S):
    """(start, end) sample spans cut at the quietest 0.5 s near each target."""
    hop = rate // 50  # 20 ms frames
    n = len(x) // hop
    if n == 0:
        return [(0, len(x))] if len(x) else []
    frames = x[: n * hop].astype(np.float32).reshape(n, hop)
    energy = np.sqrt(np.mean(frames * frames, axis=1) + 1e-12)
    energy = np.convolve(energy, np.ones(25) / 25, mode="same")
    cuts, pos = [0], 0
    while len(x) - pos > (target + slack) * rate:
        lo = (pos + int((target - slack) * rate)) // hop
        hi = (pos + int((target + slack) * rate)) // hop
        pos = (lo + int(np.argmin(energy[lo:hi]))) * hop
        cuts.append(pos)
    cuts.append(len(x))
    return list(zip(cuts[:-1], cuts[1:]))


def words_from_tokens(tokens, timestamps, offset):
    """Join subword tokens into words; a leading space starts a new word."""
    words = []
    for tok, ts in zip(tokens or [], timestamps or []):
        t = ts + offset
        if tok.startswith(" ") or not words:
            text = tok.strip()
            if text:
                words.append({"start": t, "end": t + FRAME_S, "text": text})
            continue
        words[-1]["text"] += tok
        words[-1]["end"] = t + FRAME_S
    return words


def segments_from_words(words):
    segs, cur = [], []
    for i, w in enumerate(words):
        cur.append(w)
        nxt = words[i + 1] if i + 1 < len(words) else None
        if (
            nxt is None
            or w["text"].endswith(SENTENCE_END)
            or nxt["start"] - w["end"] > PAUSE_S
            or nxt["end"] - cur[0]["start"] > MAX_SEGMENT_S
        ):
            segs.append({
                "start": round(cur[0]["start"], 3),
                "end": round(cur[-1]["end"], 3),
                "text": " " + " ".join(x["text"] for x in cur),
            })
            cur = []
    return segs


# Function words that are common in English and absent from Spanish,
# Portuguese, French, Italian and German sentences. A Spanish sentence with an
# English noun in it ("el deploy", "un ticket") does not trip this; a
# translated one ("we're going to have the transfer") does.
ENGLISH_MARKERS = frozenset(
    "the and of to is that we have with this for are but it's you they what there "
    "going think would can will our which i'm don't was be about so just like".split()
)
RETRY_CONTEXT_S = 30.0
MIN_PIECE_S = 15.0
MAX_DEPTH = 3
# Never trade speech for language: a candidate must keep this share of the
# words of the fullest one, or it is not a fix but a loss.
KEEP_WORDS = 0.8


def english_words(words):
    """Words in segments that read as English (>=4 words, >=25 % markers)."""
    n = 0
    for seg in segments_from_words(words):
        toks = [t.strip(".,;:?!¿¡\"()").lower() for t in seg["text"].split()]
        toks = [t for t in toks if t]
        if len(toks) >= 4 and sum(t in ENGLISH_MARKERS for t in toks) / len(toks) >= 0.25:
            n += len(toks)
    return n


def english_share(words):
    return english_words(words) / len(words) if words else 0.0


def least_english(cands):
    """The candidate with the smallest English share among those that kept
    most of the speech. The floor comes from the fullest candidate, not the
    first: a drifted chunk is the condensed one."""
    most = max(len(c) for c in cands)
    full = [c for c in cands if len(c) >= KEEP_WORDS * most]
    return min(full, key=english_share)


def quietest(x, rate, np, lo, hi):
    """Sample index of the quietest 0.5 s between lo and hi."""
    hop = rate // 50
    n = (hi - lo) // hop
    if n < 30:
        return (lo + hi) // 2
    frames = x[lo:lo + n * hop].astype(np.float32).reshape(n, hop)
    energy = np.convolve(np.sqrt(np.mean(frames * frames, axis=1) + 1e-12), np.ones(25) / 25, mode="same")
    return lo + (int(np.argmin(energy[12:-12])) + 12) * hop


def recognize_words(model, x, rate, np, start, end, keep=None):
    """Decode x[start:end]; with keep=(a, b), return only the words inside it."""
    start, end = max(0, start), min(len(x), end)
    res = model.recognize(x[start:end].astype(np.float32) / 32768.0, sample_rate=rate)
    words = words_from_tokens(res.tokens, res.timestamps, start / rate)
    if keep:
        lo, hi = keep[0] / rate, keep[1] / rate
        words = [w for w in words if lo <= w["start"] < hi]
    return words


def retranscribe(model, x, rate, np, start, end, words, depth=0):
    """Re-decode a stretch that came out in English, cut differently: once
    with context on both sides, and in halves at the quietest point, halves
    that drift again being split again down to MIN_PIECE_S."""
    ctx = int(RETRY_CONTEXT_S * rate)
    wider = recognize_words(model, x, rate, np, start - ctx, end + ctx, (start, end))
    cands = [words, wider]
    if english_share(wider) == 0 and len(wider) >= KEEP_WORDS * max(len(words), 1):
        return wider
    if end - start >= 2 * MIN_PIECE_S * rate and depth < MAX_DEPTH:
        third = (end - start) // 3
        mid = quietest(x, rate, np, start + third, end - third)
        halves = []
        for a, b in ((start, mid), (mid, end)):
            half = recognize_words(model, x, rate, np, a, b)
            if english_words(half):
                half = retranscribe(model, x, rate, np, a, b, half, depth + 1)
            halves += half
        cands.append(halves)
    return least_english(cands)


def is_oom(exc):
    s = str(exc).lower()
    return "out of memory" in s or "failed to allocate memory" in s


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--audio")
    ap.add_argument("--model-dir", required=True)
    ap.add_argument("--device", default="cuda")
    ap.add_argument("--language", default="")
    ap.add_argument("--download-only", action="store_true")
    args = ap.parse_args()

    try:
        import numpy as np
        import onnxruntime as ort
        import onnx_asr
    except ImportError as exc:
        emit({"type": "error", "message": f"parakeet runtime not installed: {exc}"})
        return 1

    try:
        ensure_model(args.model_dir)
    except Exception as exc:  # noqa: BLE001 - surface any download failure verbatim
        emit({"type": "error", "message": f"download model: {exc}"})
        return 1
    if args.download_only:
        return 0
    if not args.audio:
        emit({"type": "error", "message": "--audio is required"})
        return 1

    cuda = args.device == "cuda"
    ort.set_default_logger_severity(3)  # graph-optimisation warnings are not actionable
    providers = ["CPUExecutionProvider"]
    if cuda:
        # onnxruntime-gpu ships without CUDA/cuDNN; the pip wheels installed
        # next to it are only found if they are loaded explicitly.
        if hasattr(ort, "preload_dlls"):
            try:
                ort.preload_dlls()
            except Exception:  # noqa: BLE001 - a system CUDA install may still work
                pass
        providers = ["CUDAExecutionProvider", "CPUExecutionProvider"]

    emit({"type": "status", "stage": "loading parakeet-tdt-0.6b-v3"})
    try:
        model = onnx_asr.load_model(MODEL_NAME, args.model_dir, providers=providers).with_timestamps()
    except Exception as exc:  # noqa: BLE001
        prefix = "CUDA out of memory: " if cuda and is_oom(exc) else ""
        emit({"type": "error", "message": f"load model: {prefix}{exc}"})
        return 1

    emit({"type": "status", "stage": "reading audio"})
    try:
        x, rate = load_audio(args.audio, np)
    except Exception as exc:  # noqa: BLE001
        emit({"type": "error", "message": f"read audio: {exc}"})
        return 1

    emit({"type": "info", "duration": len(x) / rate if rate else 0.0, "language": args.language})
    target = args.language.strip().lower()
    english_chunks = done = 0
    try:
        for start, end in split(x, rate, np):
            words = recognize_words(model, x, rate, np, start, end)
            # Off when the meeting is meant to be English: configured so, or,
            # on auto, because most of what has been heard so far came out so.
            guard = target != "en" and (target != "" or english_chunks * 2 <= done)
            if guard and english_words(words):
                emit({"type": "status", "stage": "re-checking a passage that came out in English"})
                words = retranscribe(model, x, rate, np, start, end, words)
            done += 1
            if english_share(words) > 0.5:
                english_chunks += 1
            for seg in segments_from_words(words):
                emit({"type": "segment", **seg})
    except Exception as exc:  # noqa: BLE001
        # onnxruntime words a GPU allocation failure as "Failed to allocate
        # memory", without naming CUDA; say it, so anoted queues for VRAM
        # instead of treating it as a broken install.
        prefix = "CUDA out of memory: " if cuda and is_oom(exc) else ""
        emit({"type": "error", "message": f"transcribe: {prefix}{exc}"})
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
