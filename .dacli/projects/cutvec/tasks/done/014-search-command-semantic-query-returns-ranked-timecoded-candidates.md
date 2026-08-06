---
id: t-01KZ9WMYTERKPZHFHV79QMQS37
kind: task
created: 2026-08-05T21:18:10Z
created_by: a-root
owner: a-root
priority: must
estimate: "{optimistic: 3, probable: 5, pessimistic: 8}"
---
# search command: semantic query returns ranked timecoded candidates
## So that
an agent asks for a moment by meaning and gets timecodes + scores
## Acceptance
- [x] cutvec search <project> '<query>' --json returns ranked {start,end,text,score}
- [x] results are deterministic for a fixed query+index
## Log
- 2026-08-06T06:00:12Z claimed by a-fixer-2c1f26
- 2026-08-06T07:12:57Z accepted by a-root
- 2026-08-06T07:12:57Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-06T07:12:57Z completed by a-root
- 2026-08-06T08:47:35Z accepted by a-root
- 2026-08-06T08:47:35Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-06T08:47:35Z completed by a-root
- 2026-08-06T08:47:35Z a-fixer-fs1gss: PR opened: https://github.com/mlnomadpy/cutvec/pull/12 (event 01KZB3YQTJKQXZXD1YBVQPX1T9)
