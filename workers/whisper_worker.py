#!/usr/bin/env python3
"""cutvec transcribe worker — word-level transcription via whisper.cpp.

op: "transcribe"
params: {"source": "<path to media>"}
result: {"source", "language", "text", "words":[{"word","start","end"}]}

whisper.cpp needs 16 kHz mono PCM, so the media is first decoded with ffmpeg.
Passing `-ml 1` makes whisper emit one segment per word, which is how the
word-level start/end times the captions and search index depend on are obtained.

Model lookup order: DAVID_WHISPER_MODEL, then CUTVEC_WHISPER_MODEL (deprecated), then the ggml models podcli/whisper.cpp
commonly install under ~/Library/Application Support/podcli/models and
/opt/homebrew/share/whisper-cpp.
"""

from __future__ import annotations

import json
import os
import tempfile
from pathlib import Path

from cutvec_worker import WorkerError, log, need, require_tool, run_tool, serve

MODEL_CANDIDATES = [
    Path.home() / "Library/Application Support/podcli/models/ggml-base.bin",
    Path("/opt/homebrew/share/whisper-cpp/ggml-base.bin"),
    Path.home() / ".cache/whisper/ggml-base.bin",
]


def find_model() -> str:
    env = os.environ.get("DAVID_WHISPER_MODEL")
    if not env:
        legacy = os.environ.get("CUTVEC_WHISPER_MODEL")
        if legacy:
            log(
                {
                    "level": "warn",
                    "msg": "CUTVEC_WHISPER_MODEL is deprecated, rename to DAVID_WHISPER_MODEL",
                    "deprecated": "CUTVEC_WHISPER_MODEL",
                    "canonical": "DAVID_WHISPER_MODEL",
                }
            )
            env = legacy
    if env:
        if not Path(env).exists():
            raise WorkerError(
                "worker.missing",
                f"DAVID_WHISPER_MODEL points at a file that does not exist: {env}",
                hint="set DAVID_WHISPER_MODEL to a ggml whisper model (.bin)",
            )
        return env
    for cand in MODEL_CANDIDATES:
        if cand.exists():
            return str(cand)
    raise WorkerError(
        "worker.missing",
        "no ggml whisper model found",
        hint="set DAVID_WHISPER_MODEL to a ggml-*.bin, e.g. from "
        "https://huggingface.co/ggerganov/whisper.cpp",
    )


def to_wav16k(ffmpeg: str, source: str, dest: str) -> None:
    """Decode any media to the 16 kHz mono PCM whisper.cpp requires."""
    run_tool(
        [
            ffmpeg,
            "-nostdin",
            "-y",
            "-i",
            source,
            "-vn",
            "-ac",
            "1",
            "-ar",
            "16000",
            "-c:a",
            "pcm_s16le",
            dest,
        ],
        "ffmpeg audio decode",
    )


def words_from_whisper_json(doc: dict) -> tuple[list[dict], str, str]:
    """Flatten whisper.cpp's -ml 1 segments into words with second-precision times."""
    words: list[dict] = []
    for seg in doc.get("transcription", []):
        text = (seg.get("text") or "").strip()
        if not text:
            continue
        offs = seg.get("offsets") or {}
        # whisper.cpp reports offsets in milliseconds.
        start = float(offs.get("from", 0)) / 1000.0
        end = float(offs.get("to", 0)) / 1000.0
        words.append({"word": text, "start": round(start, 3), "end": round(end, 3)})
    language = (doc.get("result") or {}).get("language", "")
    full = " ".join(w["word"] for w in words)
    return words, language, full


def handle(params: dict) -> dict:
    source = need(params, "source")
    if not Path(source).exists():
        raise WorkerError(
            "input.invalid",
            f"source does not exist: {source}",
            hint="ingest the media first, then transcribe",
        )
    ffmpeg = require_tool("ffmpeg", "brew install ffmpeg")
    whisper = require_tool(
        "whisper-cli",
        "brew install whisper-cpp (provides whisper-cli)",
    )
    model = find_model()

    with tempfile.TemporaryDirectory() as tmp:
        wav = os.path.join(tmp, "audio16k.wav")
        log(f"decoding {source} -> 16kHz mono wav")
        to_wav16k(ffmpeg, source, wav)

        out_prefix = os.path.join(tmp, "out")
        log(f"transcribing with whisper.cpp model={Path(model).name}")
        run_tool(
            [
                whisper,
                "-m",
                model,
                "-f",
                wav,
                "-ml",
                "1",  # one word per segment -> word-level timing
                "-oj",
                "-of",
                out_prefix,
                "-np",
                "-pp",
            ],  # no prints / no progress on stdout
            "whisper-cli transcription",
        )
        json_path = out_prefix + ".json"
        if not Path(json_path).exists():
            raise WorkerError(
                "worker.protocol",
                "whisper-cli did not write its JSON output",
                hint="check the whisper-cli invocation and model file",
                retryable=True,
            )
        doc = json.loads(Path(json_path).read_text())

    words, language, text = words_from_whisper_json(doc)
    if not words:
        raise WorkerError(
            "worker.empty",
            "transcription produced no words",
            hint="check that the media actually contains speech",
        )
    log(f"transcribed {len(words)} words, language={language or 'unknown'}")
    return {"language": language, "text": text, "words": words}


if __name__ == "__main__":
    serve("transcribe", handle)
