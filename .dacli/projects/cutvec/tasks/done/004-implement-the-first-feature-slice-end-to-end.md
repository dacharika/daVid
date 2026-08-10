---
id: t-01KZ91JE5AS802FSFK5TDCQQTK
kind: task
created: 2026-08-05T13:24:56Z
created_by: a-root
owner: a-root
priority: must
estimate: "{optimistic: 4, probable: 8, pessimistic: 16}"
depends_on: "[t-01KZ91JE4ST8JP3ENXSMSW6Y2Q]"
---
# Implement the first feature slice end to end
## So that
the spec is proven by running code rather than by agreement
## Context
Pick the single entry point named first in cutvec's Spec section and carry one request from that entry point to a returned result.
## Acceptance
- [x] One entry point named in the project Spec runs from input to returned result
- [x] `go test ./...` covers that path with at least one test that fails when the path breaks
## Log
- 2026-08-05T14:38:53Z claimed by a-fixer-zqmn1k
- 2026-08-05T21:16:48Z a-fixer-zqmn1k: PR opened: https://github.com/mlnomadpy/cutvec/pull/2 (event 01KZ96384MERCBW4D5VC75BC44)
- 2026-08-05T21:21:07Z accepted by a-root
- 2026-08-05T21:21:07Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-05T21:21:07Z completed by a-root
- 2026-08-05T21:34:04Z status done proposed by a-fixer-zqmn1k, applied (event 01KZ964H95ZBKPABSWW7E6NJD4)
- 2026-08-10T16:54:59Z accepted by a-root
- 2026-08-10T16:54:59Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T16:54:59Z deliverable: no dacli/004-implement-the-first-feature-slice-end-to-end branch — nothing to check against trunk
- 2026-08-10T16:54:59Z completed by a-root
- 2026-08-10T17:20:43Z accepted by a-root
- 2026-08-10T17:20:43Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T17:20:43Z deliverable: no dacli/004-implement-the-first-feature-slice-end-to-end branch — nothing to check against trunk
- 2026-08-10T17:20:43Z completed by a-root
- 2026-08-10T17:35:36Z accepted by a-root
- 2026-08-10T17:35:36Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T17:35:36Z deliverable: no dacli/004-implement-the-first-feature-slice-end-to-end branch — nothing to check against trunk
- 2026-08-10T17:35:36Z completed by a-root
- 2026-08-10T17:56:22Z accepted by a-root
- 2026-08-10T17:56:22Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T17:56:22Z deliverable: no dacli/004-implement-the-first-feature-slice-end-to-end branch — nothing to check against trunk
- 2026-08-10T17:56:22Z completed by a-root
- 2026-08-10T18:10:45Z accepted by a-root
- 2026-08-10T18:10:45Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T18:10:45Z deliverable: no dacli/004-implement-the-first-feature-slice-end-to-end branch — nothing to check against trunk
- 2026-08-10T18:10:45Z completed by a-root
