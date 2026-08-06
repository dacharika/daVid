---
id: d-index-embeds-transcript-segments-via-a-local-embed-worker-subprocess-and
kind: note
note_kind: decision
created: 2026-08-06T04:55:23Z
created_by: a-fixer-dd5gc1
about: "[[013]]"
---
# index embeds transcript segments via a local embed worker subprocess and content-hash-caches a LanceDB index at cache/index-<hash>.json (+.lance table)
## Chose
index embeds transcript segments via a local embed worker subprocess and content-hash-caches a LanceDB index at cache/index-<hash>.json (+.lance table)
## Rejected
Add a native LanceDB Go binding / CGO dependency and embed in-process
## Because
A native LanceDB/embeddings binding needs CGO + a shipped native lib + a downloaded model, which breaks 'go build ./... from a clean checkout' and 'no network/API key'. Mirroring the transcribe/ingest decisions ([[d-transcribe-keys-the-word-level-transcript...]]): the ML (local embeddings + LanceDB write) lives behind a stdin/stdout worker (request.worker or CUTVEC_INDEX_WORKER), content-hash-keyed on the source sha256 so a re-run is a pure cache hit, and go test ./... stays green via a re-exec'd fake worker with zero external toolchain.
