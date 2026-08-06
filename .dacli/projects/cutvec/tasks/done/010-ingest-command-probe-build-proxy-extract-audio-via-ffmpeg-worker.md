---
id: t-01KZ9WMYQCA4X9CGWMZSHSRBZ0
kind: task
created: 2026-08-05T21:18:10Z
created_by: a-root
owner: a-root
priority: must
estimate: "{optimistic: 3, probable: 5, pessimistic: 8}"
---
# ingest command: probe + build proxy + extract audio via ffmpeg worker
## So that
downstream steps run on a fast proxy and clean audio, cached by content hash
## Acceptance
- [x] cutvec ingest <video> --json returns handles for proxy and audio plus probe metadata
- [x] artifacts are cached and re-ingest is a no-op hit
## Log
- 2026-08-05T23:41:01Z claimed by a-fixer-andad9
- 2026-08-06T00:14:52Z a-fixer-andad9: PR opened: https://github.com/mlnomadpy/cutvec/pull/7 (event 01KZA5D8GEDN5QG3Z4NG9FJ04Y)
- 2026-08-06T00:14:56Z accepted by a-root
- 2026-08-06T00:14:56Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-06T00:14:56Z completed by a-root
