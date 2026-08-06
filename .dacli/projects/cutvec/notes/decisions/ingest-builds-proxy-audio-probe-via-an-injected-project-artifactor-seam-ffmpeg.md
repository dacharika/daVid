---
id: d-ingest-builds-proxy-audio-probe-via-an-injected-project-artifactor-seam-ffmpeg
kind: note
note_kind: decision
created: 2026-08-05T23:45:05Z
created_by: a-fixer-andad9
about: "[[010]]"
---
# ingest builds proxy/audio/probe via an injected project.Artifactor seam; ffmpeg adapter in cmd invokes the worker op=ingest, tested with the re-exec'd fake worker
## Chose
ingest builds proxy/audio/probe via an injected project.Artifactor seam; ffmpeg adapter in cmd invokes the worker op=ingest, tested with the re-exec'd fake worker
## Rejected
Put ffmpeg/exec logic inside the project package, or require python3+ffmpeg on PATH in go test
## Because
Keeps internal/project pure state (no worker import), lets project.Ingest(path,nil) preserve task 008's content-hash-only recording, and reuses task 009's TestHelperProcess fake-worker pattern so go test ./... stays green with zero external toolchain in CI
