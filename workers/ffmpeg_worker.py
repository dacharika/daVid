#!/usr/bin/env python3
"""daVid ingest worker — probe a source and build the derived media ingest caches.

op: "probe"
params: {"source": "<path to media>"}
result: {"width", "height", "duration", "fps"}

op: "proxy"
params: {"source": "<path to media>", "dest": "<path to write>", "height": N}
result: {"output", "bytes"}

op: "audio"
params: {"source": "<path to media>", "dest": "<path to write>"}
result: {"output", "bytes"}

ingest records the probe on the source so framing can compute a real crop window
instead of assuming one. width/height come from the first video stream; duration
prefers the container duration and falls back to the stream's; fps is the video
stream's average frame rate, parsed from ffprobe's "num/den" rational.

proxy and audio exist so the expensive steps do not re-read the original: a 4K
master is ruinous to seek through repeatedly, and transcription wants samples,
not pixels. Both are written to a caller-chosen path — ingest keys them on the
source's content hash — so a re-ingest of unchanged bytes is a pure cache hit
that runs neither.
"""

from __future__ import annotations

import json
from pathlib import Path

from david_worker import WorkerError, log, need, require_tool, run_tool, serve_ops


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


def require_source(params: dict, what: str) -> str:
    """Resolve the source path, failing the same way for every op."""
    source = need(params, "source")
    if not Path(source).exists():
        raise WorkerError(
            "input.invalid",
            f"source does not exist: {source}",
            hint=f"ingest the media first, then {what}",
        )
    return source


def video_size(path: Path) -> tuple[int, int]:
    """Measure a file's video dimensions with ffprobe."""
    ffprobe = require_tool("ffprobe", "brew install ffmpeg (provides ffprobe)")
    proc = run_tool(
        [
            ffprobe,
            "-v",
            "error",
            "-select_streams",
            "v:0",
            "-show_entries",
            "stream=width,height",
            "-of",
            "json",
            str(path),
        ],
        "ffprobe size",
    )
    streams = json.loads(proc.stdout or "{}").get("streams") or [{}]
    return int(streams[0].get("width") or 0), int(streams[0].get("height") or 0)


def publish(tmp: Path, dest: Path, what: str) -> dict:
    """Move a finished render onto its final path and report the handle.

    ffmpeg is told to write a .tmp sibling and the result is renamed into place,
    so a crash or a kill leaves the discarded temp file rather than a truncated
    artifact at the cache path — which the next ingest would otherwise return as
    a completed cache hit. Same reasoning as the render publish step.
    """
    if not tmp.exists() or tmp.stat().st_size == 0:
        raise WorkerError(
            "worker.crashed",
            f"{what} produced no output at {tmp}",
            hint=f"run the {what} ffmpeg command manually to see why",
            retryable=True,
        )
    tmp.replace(dest)
    size = dest.stat().st_size
    log(f"wrote {dest} ({size} bytes)")
    return {"output": str(dest), "bytes": size}


def handle_proxy(params: dict) -> dict:
    """Build a small, fast-seeking proxy of the same duration as the source."""
    source = require_source(params, "build a proxy")
    dest = Path(need(params, "dest"))
    # 540p by default: enough to see framing and read a face, small enough that
    # repeated seeks are cheap. Odd widths break some encoders, so -2 keeps the
    # aspect ratio while forcing an even width.
    height = int(params.get("height") or 540)
    ffmpeg = require_tool("ffmpeg", "brew install ffmpeg")
    dest.parent.mkdir(parents=True, exist_ok=True)
    tmp = dest.with_suffix(dest.suffix + ".tmp")

    log(f"building {height}p proxy of {source}")
    run_tool(
        [
            ffmpeg,
            "-y",
            "-nostdin",
            "-loglevel",
            "error",
            "-i",
            source,
            "-vf",
            f"scale=-2:{height}",
            "-c:v",
            "libx264",
            "-preset",
            "veryfast",
            "-crf",
            "28",
            # A keyframe every second keeps seeking responsive, which is the
            # whole point of a proxy.
            "-g",
            "30",
            "-c:a",
            "aac",
            "-b:a",
            "128k",
            "-movflags",
            "+faststart",
            # The temp path ends in .tmp, so ffmpeg cannot infer the container
            # from the extension and refuses with "Unable to choose an output
            # format". Name it explicitly rather than renaming after the fact.
            "-f",
            "mp4",
            str(tmp),
        ],
        "ffmpeg proxy",
    )
    res = publish(tmp, dest, "proxy")
    # Report the dimensions actually produced rather than the ones requested.
    # A crop computed against assumed proxy dimensions but applied to real ones
    # renders successfully and frames nothing, so the caller needs the truth:
    # -2 rounds the width to keep it even, and the height is only honoured when
    # the source is taller than it.
    w, h = video_size(dest)
    res["width"], res["height"] = w, h
    log(f"proxy is {w}x{h}")
    return res


def handle_audio(params: dict) -> dict:
    """Extract the audio track on its own, as 16 kHz mono PCM."""
    source = require_source(params, "extract audio")
    dest = Path(need(params, "dest"))
    ffmpeg = require_tool("ffmpeg", "brew install ffmpeg")
    dest.parent.mkdir(parents=True, exist_ok=True)
    tmp = dest.with_suffix(dest.suffix + ".tmp")

    log(f"extracting audio from {source}")
    run_tool(
        [
            ffmpeg,
            "-y",
            "-nostdin",
            "-loglevel",
            "error",
            "-i",
            source,
            # 16 kHz mono PCM is what whisper wants; handing it anything else
            # just makes it resample internally.
            "-vn",
            "-acodec",
            "pcm_s16le",
            "-ar",
            "16000",
            "-ac",
            "1",
            # As above: .tmp carries no format, so name the container.
            "-f",
            "wav",
            str(tmp),
        ],
        "ffmpeg audio extract",
    )
    return publish(tmp, dest, "audio")


def handle(params: dict) -> dict:
    source = require_source(params, "probe")
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
    serve_ops({"probe": handle, "proxy": handle_proxy, "audio": handle_audio})
