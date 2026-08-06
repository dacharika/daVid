---
id: d-render-keys-the-output-on-the-built-timeline-s-sha256-quality-tier-in-cache
kind: note
note_kind: decision
created: 2026-08-06T07:23:10Z
created_by: a-fixer-0kc19a
about: "[[017]]"
---
# render keys the output on the built timeline's sha256 + quality tier in cache/render-<hash>-<quality>.mp4; the melt worker is a configurable subprocess (request.worker or CUTVEC_MELT_WORKER), tested via a re-exec'd fake worker
## Chose
render keys the output on the built timeline's sha256 + quality tier in cache/render-<hash>-<quality>.mp4; the melt worker is a configurable subprocess (request.worker or CUTVEC_MELT_WORKER), tested via a re-exec'd fake worker
## Rejected
Shell out to melt/ffmpeg directly from Go, or key the render only on the timeline path (re-render every call)
## Because
Mirrors the transcribe decision: content-hash keying makes a re-run a pure cache hit (deterministic/idempotent/resumable per the AX contract); keying on timeline hash + quality lets proxy and final coexist without collision; the stdin/stdout worker keeps melt out of the Go core so go test ./... stays green with a helper-process fake worker (no melt/ffmpeg on PATH).
