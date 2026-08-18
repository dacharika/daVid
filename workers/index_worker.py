#!/usr/bin/env python3
"""cutvec index worker — embed transcript segments into a local LanceDB table.

op: "index"
params: {"transcript": "<path to transcript.json>", "table": "<path to .lance table>"}
result: {"source", "table", "segments", "dim", "model"}

Word-level timestamps are too fine to retrieve on ("the", "a"), so words are
grouped into overlapping segments of a few seconds each: long enough to carry a
complete thought, short enough that a hit points at a usable clip. Each segment
keeps its start/end so a search hit is directly a timecode an editor can cut on.

The table is a LanceDB directory on disk — embedded, file-based, no server.
"""

from __future__ import annotations

import json
from pathlib import Path

import embedder
from david_worker import WorkerError, log, need, require_module, serve_ops

SEG_MAX_SEC = 12.0  # a segment never spans more than this
SEG_MIN_SEC = 4.0  # ...and is closed once it is at least this long
SEG_OVERLAP = 1  # words of overlap so a thought split across a boundary
# is still retrievable from both sides


def segment(words: list[dict]) -> list[dict]:
    """Group word-level tokens into retrievable, timecoded segments."""
    segments: list[dict] = []
    cur: list[dict] = []
    for w in words:
        cur.append(w)
        span = cur[-1]["end"] - cur[0]["start"]
        ends_sentence = w.get("word", "").strip().endswith((".", "?", "!"))
        if span >= SEG_MAX_SEC or (span >= SEG_MIN_SEC and ends_sentence):
            segments.append(
                {
                    "start": round(cur[0]["start"], 3),
                    "end": round(cur[-1]["end"], 3),
                    "text": " ".join(x["word"].strip() for x in cur).strip(),
                }
            )
            cur = cur[-SEG_OVERLAP:] if SEG_OVERLAP else []
    if cur:
        tail = {
            "start": round(cur[0]["start"], 3),
            "end": round(cur[-1]["end"], 3),
            "text": " ".join(x["word"].strip() for x in cur).strip(),
        }
        # avoid emitting a trailing fragment that is only the overlap
        if not segments or tail["text"] != segments[-1]["text"]:
            segments.append(tail)
    return [s for s in segments if s["text"]]


def handle(params: dict) -> dict:
    transcript_path = need(params, "transcript")
    table_path = need(params, "table")
    if not Path(transcript_path).exists():
        raise WorkerError(
            "input.invalid",
            f"transcript does not exist: {transcript_path}",
            hint="run `cutvec transcribe` before `cutvec index`",
        )
    lancedb = require_module("lancedb", "pip install lancedb")

    doc = json.loads(Path(transcript_path).read_text())
    words = doc.get("words") or []
    if not words:
        raise WorkerError(
            "input.invalid",
            "transcript has no words to index",
            hint="re-run transcribe: an empty transcript cannot be searched",
        )
    segments = segment(words)
    log(f"{len(words)} words -> {len(segments)} segments")

    emb = embedder.load()
    vectors = emb.encode([s["text"] for s in segments])
    dim = len(vectors[0]) if vectors else 0
    if not dim:
        raise WorkerError(
            "worker.crashed",
            "embedding backend returned no vectors",
            hint="check the embedding model downloaded correctly",
            retryable=True,
        )

    # LanceDB wants a database directory plus a named table; the core hands us a
    # path for the table itself, so its parent is the database.
    table_dir = Path(table_path)
    table_dir.parent.mkdir(parents=True, exist_ok=True)
    db = lancedb.connect(str(table_dir.parent))
    name = table_dir.name.removesuffix(".lance")

    rows = [
        {"vector": v, "start": s["start"], "end": s["end"], "text": s["text"]}
        # strict: a length mismatch means the embedder dropped a segment, and
        # silently truncating would lose it from the index with no error.
        for s, v in zip(segments, vectors, strict=True)
    ]
    db.create_table(name, data=rows, mode="overwrite")
    log(f"wrote {len(rows)} segments (dim {dim}) to {table_dir}")

    return {
        "source": doc.get("source", ""),
        "table": str(table_dir),
        "segments": len(rows),
        "dim": dim,
        "model": emb.label,
    }


def handle_assets(params: dict) -> dict:
    """Embed a labelled asset library so sounds can be found by description.

    Assets carry no timeline of their own, so unlike transcript segments each
    row is keyed by a reference — the path an editor will actually place — and
    the text is whatever describes it: a filename, a sidecar note, tags.
    """
    items = params.get("items") or []
    table_path = need(params, "table")
    if not items:
        raise WorkerError(
            "input.empty",
            "no assets to index",
            hint='pass items as [{"ref": "/path/to.wav", "text": "deep impact hit"}]',
        )
    lancedb = require_module("lancedb", "pip install lancedb")

    rows_in = []
    for i, it in enumerate(items):
        ref = (it.get("ref") or "").strip()
        text = (it.get("text") or "").strip()
        if not ref:
            raise WorkerError(
                "input.invalid",
                f"asset {i} has no ref",
                hint="every asset needs a ref: the path to place on a timeline",
            )
        rows_in.append((ref, text or ref))

    emb = embedder.load()
    vectors = emb.encode([t for _, t in rows_in])
    dim = len(vectors[0]) if vectors else 0
    if not dim:
        raise WorkerError(
            "worker.crashed",
            "embedding backend returned no vectors",
            hint="check the embedding model downloaded correctly",
            retryable=True,
        )

    table_dir = Path(table_path)
    table_dir.parent.mkdir(parents=True, exist_ok=True)
    db = lancedb.connect(str(table_dir.parent))
    name = table_dir.name.removesuffix(".lance")
    rows = [
        {"vector": v, "ref": ref, "text": text}
        for (ref, text), v in zip(rows_in, vectors, strict=True)
    ]
    db.create_table(name, data=rows, mode="overwrite")
    log(f"indexed {len(rows)} assets (dim {dim}) into {table_dir}")
    return {"table": str(table_dir), "assets": len(rows), "dim": dim, "model": emb.label}


if __name__ == "__main__":
    serve_ops({"index": handle, "index_assets": handle_assets})
