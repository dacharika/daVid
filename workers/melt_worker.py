#!/usr/bin/env python3
"""cutvec melt worker — render MLT timelines and report what MLT can do.

op: "render"
params: {"timeline": "<path to .mlt>", "output": "<path to write>", "quality": "proxy"|"final"}
result: {"output", "quality", "bytes"}

op: "verify"
params: {"timeline": "<path to .mlt>", "frames": N}
result: {"ok": bool, "problems": [...], "frames", "seconds"}

op: "filters"
params: {"query": "<substring>", "name": "<exact filter>", "limit": N}
result: {"filters":[{"name","description","params":[{"name","type","description"}]}], "count", "total"}

The filters op exists so an agent can discover MLT's real capability surface at
runtime — several hundred filters, including the whole avfilter set — instead of
being limited to whatever this tool hardcoded. Ask by keyword to find candidates,
then by exact name to get the parameters needed to configure one.

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
import subprocess
import time
from pathlib import Path

from cutvec_worker import WorkerError, log, need, require_tool, run_tool, serve_ops

PROFILES = {
    # (video codec args) — melt's avformat consumer takes ffmpeg-style options
    "proxy": ["vcodec=libx264", "preset=veryfast", "crf=28", "acodec=aac", "ab=128k"],
    "final": ["vcodec=libx264", "preset=medium", "crf=18", "acodec=aac", "ab=192k"],
}


def list_filters(melt: str) -> list[str]:
    """Every filter melt has registered, in its own order."""
    out = run_tool([melt, "-query", "filters"], "melt filter query").stdout
    names = []
    for line in out.splitlines():
        line = line.strip()
        if line.startswith("- "):
            names.append(line[2:].strip())
    return names


def describe_filter(melt: str, name: str) -> dict:
    """One filter's description and parameters, parsed from melt's YAML-ish output."""
    proc = subprocess.run([melt, "-query", f"filter={name}"], capture_output=True, text=True)
    info = {"name": name, "description": "", "params": []}
    current = None
    for raw in proc.stdout.splitlines():
        line = raw.strip()
        if line.startswith("description:") and not info["description"] and current is None:
            info["description"] = line.split(":", 1)[1].strip().strip('"')
        elif line.startswith("- identifier:"):
            current = {"name": line.split(":", 1)[1].strip(), "type": "", "description": ""}
            info["params"].append(current)
        elif current is not None and line.startswith("type:"):
            current["type"] = line.split(":", 1)[1].strip()
        elif current is not None and line.startswith("description:") and not current["description"]:
            current["description"] = line.split(":", 1)[1].strip().strip('"')
    return info


def handle_filters(params: dict) -> dict:
    melt = require_tool("melt", "brew install mlt (provides melt)")
    name = (params.get("name") or "").strip()
    query = (params.get("query") or "").strip().lower()
    limit = int(params.get("limit") or 40)

    everything = list_filters(melt)
    if name:
        if name not in everything:
            near = [n for n in everything if query or name.lower() in n.lower()][:8]
            raise WorkerError(
                "filter.unknown",
                f"MLT has no filter named {name!r}",
                hint=("closest names: " + ", ".join(near))
                if near
                else "call filters with a query to search the registry",
            )
        return {"filters": [describe_filter(melt, name)], "count": 1, "total": len(everything)}

    matches = [n for n in everything if query in n.lower()] if query else everything
    truncated = matches[:limit]
    # Parameters are only expanded for a focused result set: describing several
    # hundred filters would cost an agent far more tokens than it can use.
    detailed = (
        [describe_filter(melt, n) for n in truncated]
        if len(truncated) <= 12
        else [{"name": n, "description": "", "params": []} for n in truncated]
    )
    log(f"filters: {len(matches)} match of {len(everything)}; returning {len(detailed)}")
    return {"filters": detailed, "count": len(matches), "total": len(everything)}


def handle_render(params: dict) -> dict:
    timeline = need(params, "timeline")
    output = need(params, "output")
    quality = (params.get("quality") or "final").lower()

    if quality not in PROFILES:
        raise WorkerError(
            "input.invalid",
            f"unknown quality {quality!r}",
            hint="quality must be 'proxy' or 'final'",
        )
    if not Path(timeline).exists():
        raise WorkerError(
            "input.invalid",
            f"timeline does not exist: {timeline}",
            hint="build a timeline with `cutvec timeline` before rendering",
        )
    melt = require_tool(
        "melt",
        "brew install mlt (provides melt, the MLT XML renderer)",
    )
    Path(output).parent.mkdir(parents=True, exist_ok=True)

    argv = [melt, "-progress2", timeline, "-consumer", f"avformat:{output}"] + PROFILES[quality]
    log(f"rendering {Path(timeline).name} -> {output} ({quality})")
    run_tool(argv, "melt render")

    if not Path(output).exists():
        raise WorkerError(
            "worker.protocol",
            "melt reported success but wrote no output file",
            hint="check the timeline's producers resolve to readable media",
            retryable=True,
        )
    size = os.path.getsize(output)
    log(f"rendered {size} bytes")
    return {"output": output, "quality": quality, "bytes": size}


# Lines melt writes to stderr when it cannot load or play part of a timeline.
# Matched case-insensitively as substrings, because the surrounding text varies
# by MLT build and by which module failed.
LOAD_FAILURES = (
    "failed to load producer",
    "invalid document",
    "unable to load",
    "cannot open",
    "no such file",
)


def handle_verify(params: dict) -> dict:
    """Play a timeline through the null consumer to prove melt can render it.

    `--plan` validates structure in Go, which cannot know whether melt will
    actually resolve every producer and filter. This runs the real graph with no
    encoder attached, so a broken timeline fails in well under a second instead
    of after minutes of rendering.

    IMPORTANT: melt exits 0 even when a producer fails to load — measured, with
    "failed to load producer" on stderr and status 0. So this reads stderr and
    ignores the exit code for that determination. Branching on the exit code
    would report success on a broken timeline, which is worse than no check at
    all because it manufactures confidence.
    """
    timeline = need(params, "timeline")
    if not Path(timeline).exists():
        raise WorkerError(
            "input.invalid",
            f"timeline does not exist: {timeline}",
            hint="build a timeline with `cutvec timeline` before verifying",
        )
    # Bounded on purpose: this answers "does it load and start", not "does every
    # frame of the whole edit render", which would cost as much as rendering.
    frames = int(params.get("frames") or 10)
    if frames < 1:
        frames = 1
    melt = require_tool("melt", "brew install mlt (provides melt, the MLT XML renderer)")

    started = time.monotonic()
    log(f"verifying {Path(timeline).name} over {frames} frames")
    proc = subprocess.run(
        [melt, timeline, "-consumer", "null", f"out={frames - 1}"],
        capture_output=True,
        text=True,
    )
    elapsed = round(time.monotonic() - started, 3)

    blob = f"{proc.stderr or ''}\n{proc.stdout or ''}"
    problems = sorted(
        {
            line.strip()
            for line in blob.splitlines()
            if any(sig in line.lower() for sig in LOAD_FAILURES)
        }
    )
    # A non-zero exit is still a failure worth reporting; it is only the
    # *absence* of one that proves nothing.
    if proc.returncode != 0 and not problems:
        tail = (proc.stderr or "").strip().splitlines()[-3:]
        problems = [f"melt exited {proc.returncode}: " + " / ".join(tail)]

    ok = not problems
    log(f"verify {'ok' if ok else 'FAILED'} in {elapsed}s")
    return {"ok": ok, "problems": problems, "frames": frames, "seconds": elapsed}


if __name__ == "__main__":
    serve_ops({"render": handle_render, "filters": handle_filters, "verify": handle_verify})
