---
id: d-index-mirrors-transcribe-op-index-embed-worker-writes-cache-index-hash-lance-go
kind: note
note_kind: decision
created: 2026-08-06T08:21:17Z
created_by: a-fixer-e29tn6
about: "[[013]]"
---
# index mirrors transcribe: op=index embed worker writes cache/index-<hash>.lance, Go caches metadata at cache/index-<hash>.json keyed on source sha256; requires an existing transcript (transcript.missing precondition)
## Chose
index mirrors transcribe: op=index embed worker writes cache/index-<hash>.lance, Go caches metadata at cache/index-<hash>.json keyed on source sha256; requires an existing transcript (transcript.missing precondition)
## Rejected
Pass segments from Go and embed in-process via a CGO LanceDB binding
## Because
Mirrors the transcribe/render seam per [[d-index-embeds-transcript-segments-via-a-local-embed-worker-subprocess-and]]: the ML (local embeddings + LanceDB write) stays behind a stdin/stdout worker (request.worker or CUTVEC_INDEX_WORKER), content-hash-keyed so a re-run is a pure cache hit; go test stays green via a re-exec'd fake worker with no python/lancedb on PATH; the worker verifies-wrote-the-.lance-table like render verifies its output.
