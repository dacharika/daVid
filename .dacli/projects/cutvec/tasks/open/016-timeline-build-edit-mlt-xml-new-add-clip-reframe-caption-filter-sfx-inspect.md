---
id: t-01KZ9WMYVX9C822TRFHE7EEA0Z
kind: task
created: 2026-08-05T21:18:10Z
created_by: a-root
owner: a-root
priority: must
estimate: "{optimistic: 8, probable: 13, pessimistic: 20}"
---
# timeline: build/edit MLT XML (new, add-clip, reframe, caption, filter, sfx, inspect, validate)
## So that
the MLT XML timeline is the agent-editable artifact
## Acceptance
- [x] cutvec timeline new/add-clip/... mutate a valid .mlt file
- [x] cutvec timeline inspect --json returns the timeline structure; validate reports errors with hints
## Log
- 2026-08-06T07:20:51Z accepted by a-root
- 2026-08-06T07:20:51Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-06T07:20:51Z completed by a-root
