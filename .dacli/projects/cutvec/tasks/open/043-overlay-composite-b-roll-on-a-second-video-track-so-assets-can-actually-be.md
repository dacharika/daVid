---
id: t-01KZS5G8MXH4AM54T3NRFS4XER
kind: task
created: 2026-08-11T19:41:30Z
created_by: a-root
owner: a-root
priority: should
estimate: "{optimistic: 3, probable: 5, pessimistic: 8}"
---
# overlay: composite b-roll on a second video track so assets can actually be placed
## So that
`assets search` stops promising something the timeline cannot express

## The gap
The `assets` tool summary says it "returns ranked refs to place on a timeline". The timeline model has exactly two composition methods: `Sequence` (sequential video) and `AddAudio` (an audio track plus a mix transition). There is no video overlay.

So retrieved b-roll can only be placed as a full cutaway that replaces the interview audio too, because a producer in a playlist carries its own A/V. Cutting to b-roll while the answer keeps playing — the normal reason to want b-roll — is not expressible.

## Acceptance
- [ ] the timeline model can add a second video track carrying a producer at an offset, composited over the main track via `composite`/`qtblend` (both confirmed available)
- [ ] the underlying track's audio is preserved — the point is picture-only cutaway; a test asserts the main track's audio is not replaced
- [ ] `timeline` accepts b-roll placements in its request, so an agent can go from `assets search` to a placed overlay without hand-writing MLT
- [ ] a placement that runs past the end of the base timeline is rejected with a branchable code rather than silently truncated
- [ ] verified by rendering an overlay on real footage and looking at a frame inside the overlay window
- [ ] `go build ./...`, `go test ./... -count=1`, gofmt, vet clean

## Log
- 2026-08-11 filed by a-root from the MLT capability audit
- 2026-08-17T03:17:19Z claimed by a-6jyh2fmkjw
