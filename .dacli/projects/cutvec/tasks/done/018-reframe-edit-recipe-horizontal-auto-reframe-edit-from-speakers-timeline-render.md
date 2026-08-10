---
id: t-01KZ9WMYXD1EHA8GEKGMKAQMC3
kind: task
created: 2026-08-05T21:18:10Z
created_by: a-root
owner: a-root
priority: should
estimate: "{optimistic: 5, probable: 8, pessimistic: 13}"
---
# reframe-edit recipe: horizontal auto-reframe edit from speakers + timeline + render
## So that
one call reproduces the fake-multicam horizontal edit
## Acceptance
- [x] cutvec reframe-edit <project> --json produces a 1080p edit that cuts to the active speaker
## Log
- 2026-08-10T15:05:23Z accepted by a-root
- 2026-08-10T15:05:23Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T15:05:23Z deliverable: no dacli/018-reframe-edit-recipe-horizontal-auto-reframe-edit-from-speakers-timeline-render branch — nothing to check against trunk
- 2026-08-10T15:05:23Z completed by a-root
