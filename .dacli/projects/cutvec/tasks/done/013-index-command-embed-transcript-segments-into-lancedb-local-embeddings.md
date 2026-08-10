---
id: t-01KZ9WMYSPMHARS5TGB7AH9H4A
kind: task
created: 2026-08-05T21:18:10Z
created_by: a-root
owner: a-root
priority: must
estimate: "{optimistic: 3, probable: 5, pessimistic: 8}"
---
# index command: embed transcript segments into LanceDB (local embeddings)
## So that
search retrieves moments semantically instead of scanning the transcript
## Acceptance
- [x] cutvec index <project> builds a local LanceDB index of transcript segments
- [x] no network / API key required
## Log
- 2026-08-06T01:45:01Z claimed by a-fixer-dd5gc1
- 2026-08-06T06:00:09Z accepted by a-root
- 2026-08-06T06:00:09Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-06T06:00:09Z completed by a-root
- 2026-08-06T08:26:53Z a-fixer-e29tn6: PR opened: https://github.com/mlnomadpy/cutvec/pull/11 (event 01KZB2T485B04N7MXV920M27A0)
- 2026-08-06T08:35:48Z accepted by a-root
- 2026-08-06T08:35:48Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-06T08:35:48Z completed by a-root
- 2026-08-10T16:55:01Z accepted by a-root
- 2026-08-10T16:55:01Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T16:55:01Z deliverable: no dacli/013-index-command-embed-transcript-segments-into-lancedb-local-embeddings branch — nothing to check against trunk
- 2026-08-10T16:55:01Z completed by a-root
