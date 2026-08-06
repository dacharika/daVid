---
id: d-speakers-mirrors-transcribe-mediapipe-worker-subprocess-cache-speakers-hash
kind: note
note_kind: decision
created: 2026-08-06T08:08:23Z
created_by: a-fixer-72cctq
about: "[[012]]"
---
# speakers mirrors transcribe: mediapipe worker subprocess, cache/speakers-<hash>.json keyed on source sha256, FIXED two-person layout constant (project.TwoPersonLayout) set by the core not the request
## Chose
speakers mirrors transcribe: mediapipe worker subprocess, cache/speakers-<hash>.json keyed on source sha256, FIXED two-person layout constant (project.TwoPersonLayout) set by the core not the request
## Rejected
Expose a variable layout in the request while keying the cache only on the source hash (two layouts would collide on one key), or shell out to mediapipe from Go
## Because
Content-hash keying makes a re-run a pure cache hit (idempotent/resumable per the AX contract); the core fixes the layout and passes it to the worker so the recorded layout can never disagree with the cache key; the stdin/stdout worker keeps mediapipe out of the Go core so go test ./... stays green via a re-exec'd fake worker with no python/mediapipe on PATH
