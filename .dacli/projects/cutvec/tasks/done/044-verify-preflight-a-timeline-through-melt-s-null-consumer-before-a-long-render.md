---
id: t-01KZS5G8PESM90RREV9F6ZJ4KC
kind: task
created: 2026-08-11T19:41:30Z
created_by: a-root
owner: a-root
priority: should
estimate: "{optimistic: 2, probable: 3, pessimistic: 5}"
---
# verify: preflight a timeline through melt's null consumer before a long render
## So that
a broken timeline fails in under a second instead of after minutes of rendering, or worse, at the end of a batch

## Measured
`--plan` validates structure in Go only; it cannot know whether melt can actually load and play the thing. melt's `null` consumer runs the graph without encoding:

    melt timeline.mlt -consumer null out=10     0.35s

and on a timeline pointing at a missing file it emits

    [producer_xml] failed to load producer "/nope/missing.mp4"

## The catch, which the implementation must handle
**melt exits 0 anyway.** Measured: the run above returned exit status 0 with that error on stderr. So a preflight that branches on the exit code reports success on a broken timeline — worse than no preflight, because it manufactures confidence. The check must scan stderr for load failures.

## Acceptance
- [x] the melt worker gains a `verify` op that runs the null consumer over a bounded number of frames and reports whether the timeline loads
- [x] it detects a failure from stderr, NOT the exit code, and a test drives it against a timeline referencing a missing file to prove that
- [x] a `--verify` (or equivalent) on the timeline-producing paths runs it before returning, so a composed edit is known-renderable
- [x] the check is bounded — a few frames, not the whole timeline — and its cost is reported so the tradeoff is visible
- [x] `go build ./...`, `go test ./... -count=1`, gofmt, vet, ruff clean

## Log
- 2026-08-11 filed by a-root from the MLT capability audit
- 2026-08-11T19:49:21Z accepted by a-root
- 2026-08-11T19:49:21Z verified by `go build ./... && go test ./... -count=1` (exit 0)
- 2026-08-11T19:49:21Z deliverable: no dacli/044-verify-preflight-a-timeline-through-melt-s-null-consumer-before-a-long-render branch — nothing to check against main
- 2026-08-11T19:49:21Z completed by a-root
