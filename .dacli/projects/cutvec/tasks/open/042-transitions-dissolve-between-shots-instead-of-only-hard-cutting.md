---
id: t-01KZS5G8K7FFMJGE1CDN9B94JD
kind: task
created: 2026-08-11T19:41:30Z
created_by: a-root
owner: a-root
priority: should
estimate: "{optimistic: 2, probable: 3, pessimistic: 5}"
---
# transitions: dissolve between shots instead of only hard cutting
## So that
a reframe edit can breathe where the material wants it to, rather than every cut being the same abrupt one

## Verified available
`melt -query transitions` on this machine reports seven: qtblend, vqm, affine, composite, luma, mix, matte. cutvec creates exactly one — `mix`, and only for audio (internal/timeline/timeline.go, the single NewTransition call). Every visual cut in `reframe-edit` is a hard cut because nothing else is reachable.

## Acceptance
- [ ] the timeline model can place a `luma` transition spanning the boundary between two adjacent producers, with a duration
- [ ] `reframe-edit` takes an optional dissolve length, defaulting to 0 (hard cut) so today's output is unchanged — pinned by a test comparing emitted XML at the default
- [ ] a dissolve longer than either adjoining shot is rejected with a branchable code rather than producing a mangled overlap
- [ ] verified by rendering a dissolve on real footage and extracting a frame from the middle of the transition: it must show both shots blended, not one or the other
- [ ] `go build ./...`, `go test ./... -count=1`, gofmt, vet clean

## Notes
MLT expresses a dissolve as a transition over a track overlap, so this needs the two shots to overlap on the timeline rather than abut. That is the real work; the transition element itself is trivial.

## Log
- 2026-08-11 filed by a-root from the MLT capability audit
