---
id: t-01KZP3BTC6FQG43YA0WE32DG2F
kind: task
created: 2026-08-10T15:06:24Z
created_by: a-root
owner: a-root
priority: should
estimate: "{optimistic: 3, probable: 5, pessimistic: 8}"
---
# timeline edit: mutate an existing timeline instead of rebuilding it
## So that
an agent can adjust one filter on a composed edit without re-running search and detection
## Acceptance
- [x] timeline inspect returns the structure; timeline edit adds or removes a filter on a named producer
- [x] an edited timeline still validates and renders
## Log
- 2026-08-10T17:56:30Z claimed by a-fixer-0mj66x
- 2026-08-10T18:06:14Z a-fixer-0mj66x: PR opened: https://github.com/mlnomadpy/cutvec/pull/29 (event 01KZPDMD587QN9DC1HJHESVW08)
- 2026-08-10T18:10:50Z accepted by a-root (applied 1 proposal(s))
- 2026-08-10T18:10:50Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T18:10:50Z deliverable: dacli/026-timeline-edit-mutate-an-existing-timeline-instead-of-rebuilding-it is merged into trunk
- 2026-08-10T18:10:50Z completed by a-root
- 2026-08-10T18:10:53Z accepted by a-root
- 2026-08-10T18:10:53Z closed WITHOUT verification — no --verify command was given
- 2026-08-10T18:10:53Z deliverable: dacli/026-timeline-edit-mutate-an-existing-timeline-instead-of-rebuilding-it is merged into trunk
- 2026-08-10T18:10:53Z completed by a-root
- 2026-08-10T18:19:17Z accepted by a-root
- 2026-08-10T18:19:17Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T18:19:17Z deliverable: no dacli/026-timeline-edit-mutate-an-existing-timeline-instead-of-rebuilding-it branch — nothing to check against trunk
- 2026-08-10T18:19:17Z completed by a-root
