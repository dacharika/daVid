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
- [ ] timeline inspect returns the structure; timeline edit adds or removes a filter on a named producer
- [ ] an edited timeline still validates and renders
## Log
