#!/usr/bin/env python3
"""cutvec ingest worker — probe a source's intrinsic dimensions with ffprobe.

op: "probe"
params: {"source": "<path to media>"}
result: {"width", "height", "duration", "fps"}

ingest records these on the source so framing can compute a real crop window
instead of assuming one. width/height come from the first video stream; duration
prefers the container duration and falls back to the stream's; fps is the video
stream's average frame rate, parsed from ffprobe's "num/den" rational.
"""

from __future__ import annotations

import json
from pathlib import Path

from cutvec_worker import WorkerError, log, need, require_tool, run_tool, serve


def parse_rate(rate: str) -> float:
    """Turn ffprobe's "num/den" (or a plain number) into fps, guarding div-by-zero."""
    rate = (rate or "").strip()
    if not rate or rate == "0/0":
        return 0.0
    if "/" in rate:
        num, den = rate.split("/", 1)
        den_f = float(den)
        return float(num) / den_f if den_f else 0.0
    return float(rate)


def first_video_stream(doc: dict) -> dict:
    for stream in doc.get("streams", []):
        if stream.get("codec_type") == "video":
            return stream
    raise WorkerError(
        "worker.empty",
        "source has no video stream to probe",
        hint="ingest a media file that contains video",
    )


def handle(params: dict) -> dict:
    source = need(params, "source")
    if not Path(source).exists():
        raise WorkerError(
            "input.invalid",
            f"source does not exist: {source}",
            hint="ingest the media first, then probe",
        )
    ffprobe = require_tool("ffprobe", "brew install ffmpeg (provides ffprobe)")

    log(f"probing {source}")
    proc = run_tool(
        [
            ffprobe,
            "-v",
            "error",
            "-show_entries",
            "stream=codec_type,width,height,avg_frame_rate,duration:format=duration",
            "-of",
            "json",
            source,
        ],
        "ffprobe probe",
    )
    doc = json.loads(proc.stdout or "{}")
    video = first_video_stream(doc)

    width = int(video.get("width") or 0)
    height = int(video.get("height") or 0)
    fps = round(parse_rate(video.get("avg_frame_rate", "")), 3)

    # Prefer the container duration; fall back to the video stream's own.
    raw_dur = (doc.get("format") or {}).get("duration") or video.get("duration")
    try:
        duration = round(float(raw_dur), 3)
    except (TypeError, ValueError):
        duration = 0.0

    log(f"probed {width}x{height} {duration}s {fps}fps")
    return {"width": width, "height": height, "duration": duration, "fps": fps}


if __name__ == "__main__":
    serve("probe", handle)
