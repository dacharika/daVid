---
id: d-proxy-audio-probe-are-cached-under-the-source-content-hash-cache-hash-proxy-mp4
kind: note
note_kind: decision
created: 2026-08-05T23:45:11Z
created_by: a-fixer-andad9
about: "[[010]]"
---
# proxy/audio/probe are cached under the source content hash (cache/<hash>.proxy.mp4, .audio.wav, and enriched in cache/<hash>.json + manifest); re-ingest is a cache hit that returns the handles without re-running ffmpeg
## Chose
proxy/audio/probe are cached under the source content hash (cache/<hash>.proxy.mp4, .audio.wav, and enriched in cache/<hash>.json + manifest); re-ingest is a cache hit that returns the handles without re-running ffmpeg
## Rejected
A separate artifact manifest keyed by time/path, or re-running ffmpeg every ingest
## Because
Content-hash keying makes the whole ingest step (probe+proxy+audio) deterministic/idempotent/resumable per the AX contract: identical bytes always resolve to the same handles and the worker is invoked exactly once, so re-ingest is a no-op hit
