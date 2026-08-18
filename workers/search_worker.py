#!/usr/bin/env python3
"""cutvec search worker — semantic query over the indexed transcript segments.

op: "search"
params: {"query": "<what to find>", "table": "<path to .lance table>", "limit": N}
result: {"candidates":[{"start","end","text","score"}], "model"}

This is the retrieval half of the tool: an agent asks for a moment by meaning
("where she says the winners find where AI fails") and gets back ranked, cut-ready
timecodes instead of a transcript to scan.

Scores are similarities in [0,1] (higher is better), converted from LanceDB's
L2 distance, so a caller can threshold them without knowing the metric.
"""

from __future__ import annotations

from pathlib import Path

import embedder
from david_worker import WorkerError, log, need, require_module, serve_ops

DEFAULT_LIMIT = 10


def to_score(distance: float) -> float:
    """Map an L2 distance on normalised vectors to a 0..1 similarity."""
    return round(1.0 / (1.0 + max(0.0, float(distance))), 6)


def handle(params: dict) -> dict:
    query = need(params, "query")
    table_path = need(params, "table")
    limit = int(params.get("limit") or DEFAULT_LIMIT)
    if limit <= 0:
        limit = DEFAULT_LIMIT

    table_dir = Path(table_path)
    if not table_dir.exists():
        raise WorkerError(
            "index.missing",
            f"no index table at {table_path}",
            hint="run `cutvec index` before searching",
        )
    lancedb = require_module("lancedb", "pip install lancedb")

    db = lancedb.connect(str(table_dir.parent))
    name = table_dir.name.removesuffix(".lance")
    try:
        table = db.open_table(name)
    except Exception as exc:
        raise WorkerError(
            "index.missing",
            f"cannot open index table {name!r}: {exc}",
            hint="re-run `cutvec index` to rebuild the table",
        ) from exc

    emb = embedder.load()
    vector = emb.encode([query])[0]
    rows = table.search(vector).limit(limit).to_list()

    candidates = []
    for row in rows:
        candidates.append(
            {
                "start": float(row.get("start", 0.0)),
                "end": float(row.get("end", 0.0)),
                "text": row.get("text", ""),
                "score": to_score(row.get("_distance", 0.0)),
            }
        )
    log(f"query {query!r} -> {len(candidates)} candidates")
    return {"candidates": candidates, "model": emb.label}


def handle_assets(params: dict) -> dict:
    """Find assets by description, returning the refs to place on a timeline."""
    query = need(params, "query")
    table_path = need(params, "table")
    limit = int(params.get("limit") or DEFAULT_LIMIT)
    if limit <= 0:
        limit = DEFAULT_LIMIT

    table_dir = Path(table_path)
    if not table_dir.exists():
        raise WorkerError(
            "index.missing",
            f"no asset index at {table_path}",
            hint="run `cutvec assets index` before searching assets",
        )
    lancedb = require_module("lancedb", "pip install lancedb")
    db = lancedb.connect(str(table_dir.parent))
    name = table_dir.name.removesuffix(".lance")
    try:
        table = db.open_table(name)
    except Exception as exc:
        raise WorkerError(
            "index.missing",
            f"cannot open asset table {name!r}: {exc}",
            hint="re-run `cutvec assets index` to rebuild it",
        ) from exc

    emb = embedder.load()
    rows = table.search(emb.encode([query])[0]).limit(limit).to_list()
    assets = [
        {
            "ref": r.get("ref", ""),
            "text": r.get("text", ""),
            "score": to_score(r.get("_distance", 0.0)),
        }
        for r in rows
    ]
    log(f"asset query {query!r} -> {len(assets)} hits")
    return {"assets": assets, "model": emb.label}


if __name__ == "__main__":
    serve_ops({"search": handle, "search_assets": handle_assets})
