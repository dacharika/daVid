---
id: d-transcribe-keys-the-word-level-transcript-on-the-ingested-source-s-sha256-in
kind: note
note_kind: decision
created: 2026-08-05T23:44:28Z
created_by: a-fixer-sv7wx8
about: "[[011]]"
---
# transcribe keys the word-level transcript on the ingested source's sha256 in cache/transcribe-<hash>.json; the whisper worker is a configurable subprocess (request.worker or CUTVEC_WHISPER_WORKER), tested via a re-exec'd fake worker
## Chose
transcribe keys the word-level transcript on the ingested source's sha256 in cache/transcribe-<hash>.json; the whisper worker is a configurable subprocess (request.worker or CUTVEC_WHISPER_WORKER), tested via a re-exec'd fake worker
## Rejected
Re-hash the file on every call and shell out to python3 whisper directly from Go; store the transcript as a manifest blob
## Because
Content-hash keying reuses the existing project cache design so a re-run is a pure cache hit (idempotent) with no worker call; making the worker a stdin/stdout subprocess keeps the ML stack (task 010's contract) out of the Go core and lets go test ./... stay green with a helper-process fake worker instead of requiring python3+whisper on PATH.
