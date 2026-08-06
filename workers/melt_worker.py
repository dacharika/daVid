#!/usr/bin/env python3
"""cutvec render worker — render an MLT XML timeline with melt.

op: "render"
params: {"timeline": "<path to .mlt>", "output": "<path to write>", "quality": "proxy"|"final"}
result: {"output", "quality", "bytes"}

The timeline is the artifact an agent edits; this worker is the deterministic
step that turns it into a file. melt consumes the same MLT XML the timeline
commands write, so what renders is exactly what `timeline inspect` showed.

`quality` trades speed against fidelity: "proxy" for a fast look at the cut,
"final" for delivery. The worker's contract is that the output path exists when
it returns success — the core checks this and treats a missing file as a
protocol violation.
"""
from __future__ import annotations

import os
from pathlib import Path

from cutvec_worker import (WorkerError, log, need, require_tool, run_tool,
                           serve)

PROFILES = {
    # (video codec args) — melt's avformat consumer takes ffmpeg-style options
    "proxy": ["vcodec=libx264", "preset=veryfast", "crf=28", "acodec=aac", "ab=128k"],
    "final": ["vcodec=libx264", "preset=medium", "crf=18", "acodec=aac", "ab=192k"],
}


def handle(params: dict) -> dict:
    timeline = need(params, "timeline")
    output = need(params, "output")
    quality = (params.get("quality") or "final").lower()

    if quality not in PROFILES:
        raise WorkerError(
            "input.invalid", f"unknown quality {quality!r}",
            hint="quality must be 'proxy' or 'final'",
        )
    if not Path(timeline).exists():
        raise WorkerError(
            "input.invalid", f"timeline does not exist: {timeline}",
            hint="build a timeline with `cutvec timeline` before rendering",
        )
    melt = require_tool(
        "melt",
        "brew install mlt (provides melt, the MLT XML renderer)",
    )
    Path(output).parent.mkdir(parents=True, exist_ok=True)

    argv = [melt, "-progress2", timeline,
            "-consumer", f"avformat:{output}"] + PROFILES[quality]
    log(f"rendering {Path(timeline).name} -> {output} ({quality})")
    run_tool(argv, "melt render")

    if not Path(output).exists():
        raise WorkerError(
            "worker.protocol", "melt reported success but wrote no output file",
            hint="check the timeline's producers resolve to readable media",
            retryable=True,
        )
    size = os.path.getsize(output)
    log(f"rendered {size} bytes")
    return {"output": output, "quality": quality, "bytes": size}


if __name__ == "__main__":
    serve("render", handle)
