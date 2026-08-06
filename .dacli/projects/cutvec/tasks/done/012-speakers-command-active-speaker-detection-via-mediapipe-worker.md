---
id: t-01KZ9WMYRY37G50TDA6WX2MK54
kind: task
created: 2026-08-05T21:18:10Z
created_by: a-root
owner: a-root
priority: must
estimate: "{optimistic: 5, probable: 8, pessimistic: 13}"
---
# speakers command: active-speaker detection via mediapipe worker
## So that
the editor knows who is speaking when, for reframing and speaker-locked shorts
## Acceptance
- [x] cutvec speakers <project> --json returns per-time speaker labels / turns
- [x] handles a fixed two-person layout
## Log
- 2026-08-06T00:17:16Z claimed by a-fixer-wqc9fw
- 2026-08-06T01:29:39Z accepted by a-root
- 2026-08-06T01:29:39Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-06T01:29:39Z completed by a-root
- 2026-08-06T08:17:34Z accepted by a-root
- 2026-08-06T08:17:34Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-06T08:17:34Z completed by a-root
- 2026-08-06T08:17:34Z a-fixer-72cctq: PR opened: https://github.com/mlnomadpy/cutvec/pull/10 (event 01KZB1WRMWEB9FE1VQ33VS6KQT)
