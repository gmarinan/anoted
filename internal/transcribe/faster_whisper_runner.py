"""Run faster-whisper and stream results to stdout as JSON lines.

faster-whisper is a library, not a CLI, so anoted drives it through this
runner. The contract is one JSON object per line on stdout:

    {"type": "status",  "stage": "loading model"}
    {"type": "info",    "duration": 1234.5, "language": "es"}
    {"type": "segment", "start": 0.0, "end": 5.0, "text": " Hola"}
    {"type": "error",   "message": "..."}

`status` matters more than it looks: model.transcribe() decodes the whole file
and runs the VAD before it returns, which on a one-hour recording is minutes of
work with nothing to report. Without these the UI sat at 0% with no sign of
life, and the run was indistinguishable from a hang.

Emitting `duration` up front is what lets the caller show real progress:
segments carry an end timestamp, so percent is end/duration rather than a
guess. Every line is flushed immediately so progress is live, not buffered
until the process exits.
"""

import argparse
import json
import sys


def emit(obj):
    json.dump(obj, sys.stdout, ensure_ascii=False)
    sys.stdout.write("\n")
    sys.stdout.flush()


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--audio", required=True)
    ap.add_argument("--model", required=True)
    ap.add_argument("--device", default="auto")
    ap.add_argument("--compute-type", default="default")
    ap.add_argument("--language", default="")
    ap.add_argument("--beam-size", type=int, default=5)
    ap.add_argument("--vad", default="true")
    args = ap.parse_args()

    try:
        from faster_whisper import WhisperModel
    except ImportError as exc:
        emit({"type": "error", "message": f"faster-whisper not installed: {exc}"})
        return 1

    emit({"type": "status", "stage": f"loading {args.model}"})
    try:
        model = WhisperModel(
            args.model,
            device=args.device,
            compute_type=args.compute_type,
        )
    except Exception as exc:  # noqa: BLE001 - surface any load failure verbatim
        emit({"type": "error", "message": f"load model: {exc}"})
        return 1

    emit({"type": "status", "stage": "decoding audio and detecting speech"})
    try:
        segments, info = model.transcribe(
            args.audio,
            beam_size=args.beam_size,
            language=args.language or None,
            vad_filter=args.vad.lower() == "true",
        )
        emit({
            "type": "info",
            "duration": float(info.duration),
            "language": info.language or "",
        })
        # `segments` is a generator: iterating it is what performs the work, so
        # each emit below happens as that chunk of audio finishes decoding.
        for seg in segments:
            emit({
                "type": "segment",
                "start": float(seg.start),
                "end": float(seg.end),
                "text": seg.text,
            })
    except Exception as exc:  # noqa: BLE001
        emit({"type": "error", "message": f"transcribe: {exc}"})
        return 1

    return 0


if __name__ == "__main__":
    sys.exit(main())
