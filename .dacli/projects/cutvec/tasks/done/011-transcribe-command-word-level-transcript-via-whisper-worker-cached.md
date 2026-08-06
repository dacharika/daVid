---
id: t-01KZ9WMYR510C9KDHBYXFZ0AWB
kind: task
created: 2026-08-05T21:18:10Z
created_by: a-root
owner: a-root
priority: must
estimate: "{optimistic: 5, probable: 8, pessimistic: 13}"
---
# transcribe command: word-level transcript via whisper worker, cached
## So that
an agent gets timecoded words to search and to build captions
## Acceptance
- [x] cutvec transcribe <project> --json returns a handle; transcript has word-level start/end
- [x] result is cached and idempotent
## Log
- 2026-08-05T23:41:02Z claimed by a-fixer-sv7wx8
- 2026-08-06T00:14:52Z a-fixer-sv7wx8: PR opened: https://github.com/mlnomadpy/cutvec/pull/6 (event 01KZA59AQ9WKEK9K5FSJEFHMM7)
- 2026-08-06T00:14:56Z accepted by a-root
- 2026-08-06T00:14:56Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-06T00:14:56Z completed by a-root
