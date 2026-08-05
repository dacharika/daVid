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
- [ ] `go test ./...` exits 0 and runs at least one real assertion
- [ ] One deliberately failing test was observed to fail, then removed or fixed
## Log
