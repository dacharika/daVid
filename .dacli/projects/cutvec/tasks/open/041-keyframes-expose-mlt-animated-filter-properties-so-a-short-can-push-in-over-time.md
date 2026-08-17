---
id: t-01KZS5G8HN2XAZ2K7G8YF5ECR9
kind: task
created: 2026-08-11T19:41:30Z
created_by: a-root
owner: a-root
priority: should
estimate: "{optimistic: 3, probable: 5, pessimistic: 8}"
---
# keyframes: expose MLT animated filter properties so a short can push in over time
## So that
a vertical short can hold attention with a slow push instead of sitting on a locked frame for its whole length

## Verified available, not assumed
MLT animates any filter property with a `frame=value; frame=value` string. Confirmed on this machine by rendering an `affine` whose rect animates:

    <property name="transition.rect">0=0 0 960 540 1; 89=-120 -68 1200 675 1</property>

It rendered, and the first and last frames differ — the push is real, not a no-op. cutvec exposes none of this: every `Property` value it writes is a constant, and nothing in the codebase mentions animation.

## Acceptance
- [ ] a caller can give any effect property an animated value through the existing `effects` path, and the timeline writes it through to MLT verbatim
- [ ] `shorts` takes an optional push (e.g. `"push": 0.06` = zoom 6% over the clip) that animates the crop rather than adding a second filter, so the framing logic stays in one place
- [ ] the push is off by default — a locked frame stays byte-identical to today's output, proven by a test comparing the emitted XML with and without it
- [ ] `filters` reports which of a filter's parameters MLT will animate, so an agent can discover this instead of being told
- [ ] a rendered short with a push is verified by extracting the first and last frame and confirming the framing actually changed — a keyframe string that MLT silently ignores renders fine and looks static
- [ ] `go build ./...`, `go test ./... -count=1`, gofmt, vet, ruff all clean

## Notes
The failure mode to guard: a malformed animation string is not rejected by MLT, it is ignored. So "it rendered" proves nothing here; only comparing frames does.

## Log
- 2026-08-11 filed by a-root from the MLT capability audit
- 2026-08-17T03:17:19Z claimed by a-v8t287gq0d
