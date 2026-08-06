---
id: d-speakers-keys-per-source-active-speaker-turns-on-the-source-sha256-in-cache
kind: note
note_kind: decision
created: 2026-08-06T00:18:29Z
created_by: a-fixer-wqc9fw
about: "[[012]]"
---
# speakers keys per-source active-speaker turns on the source sha256 in cache/speakers-<hash>.json, uses a FIXED two-person layout constant (not a request param), via a mediapipe worker subprocess (request.worker or CUTVEC_MEDIAPIPE_WORKER)
## Chose
speakers keys per-source active-speaker turns on the source sha256 in cache/speakers-<hash>.json, uses a FIXED two-person layout constant (not a request param), via a mediapipe worker subprocess (request.worker or CUTVEC_MEDIAPIPE_WORKER)
## Rejected
Expose a variable layout in the request while keying the cache only on the source hash (a cache-key footgun where two layouts collide), or shell out to mediapipe directly from Go
## Because
Mirrors task 011's transcribe decision: content-hash keying makes re-runs pure cache hits (idempotent/resumable per the AX contract); a fixed two-person layout matches the AC exactly and keeps the cache key = source hash with no collision risk; the stdin/stdout worker keeps mediapipe out of the Go core so go test ./... stays green with a re-exec'd fake worker (no python/mediapipe on PATH)
