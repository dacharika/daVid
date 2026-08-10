---
id: t-01KZ9WMYV6CDG4QQCRNQVEND2Z
kind: task
created: 2026-08-05T21:18:10Z
created_by: a-root
owner: a-root
priority: should
estimate: "{optimistic: 3, probable: 5, pessimistic: 8}"
---
# assets library + assets search over an SFX/broll index
## So that
an agent finds the right sfx/broll by description
## Acceptance
- [x] cutvec assets search '<desc>' --json returns ranked asset handles
- [x] assets are embedded into the same vector store
## Log
- 2026-08-10T15:05:22Z accepted by a-root
- 2026-08-10T15:05:22Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T15:05:22Z deliverable: no dacli/015-assets-library-assets-search-over-an-sfx-broll-index branch — nothing to check against trunk
- 2026-08-10T15:05:22Z completed by a-root
