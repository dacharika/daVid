---
id: t-01KZ91JE4ST8JP3ENXSMSW6Y2Q
kind: task
created: 2026-08-05T13:24:56Z
created_by: a-root
owner: a-root
priority: must
estimate: "{optimistic: 1, probable: 2, pessimistic: 4}"
depends_on: "[t-01KZ91JE4HHBWVT92F6KFT22V3]"
---
# Set up the test harness
## So that
a green run means something before there is code to trust
## Context
Wire `go test ./...` so it runs from the repository root with no arguments.
## Acceptance
- [x] `go test ./...` exits 0 and runs at least one real assertion
- [x] One deliberately failing test was observed to fail, then removed or fixed
## Log
- 2026-08-05T14:33:20Z claimed by a-fixer-br37c8
- 2026-08-05T14:38:12Z accepted by a-root
- 2026-08-05T14:38:12Z verified by `go test ./...` (exit 0)
- 2026-08-05T14:38:12Z completed by a-root
- 2026-08-05T14:38:53Z status done proposed by a-fixer-br37c8, applied (event 01KZ95KC9N3T543FRS9B1QNR9Q)
