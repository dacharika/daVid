---
id: t-01KZPQYTFNYW8QR0DCBMWZ6EJ7
kind: task
created: 2026-08-10T21:06:18Z
created_by: a-go-auditor-t6n382
owner: a-go-auditor-t6n382
priority: should
estimate: "{optimistic: 1, probable: 2, pessimistic: 4}"
---
# timeline: carry Clock rounding into the next minute so a sub-millisecond time never emits an invalid :60.000 seconds field
## So that
every MLT timestamp cutvec writes is a valid clock melt can parse, not an out-of-range :60.000 that breaks the render on a boundary time
## Acceptance
- [ ] Clock (internal/timeline/timeline.go:194) rounds to whole milliseconds before decomposing h/m/s so a fractional part that rounds up carries into the next minute/hour instead of producing a seconds field of 60.000
- [ ] A new table-driven test asserts Clock(59.9996)=="00:01:00.000", Clock(119.9999)=="00:02:00.000", and Clock(3599.9999)=="01:00:00.000", and the existing 0/12.5/3671.25 and negative-clamp cases still pass
- [ ] go build ./... and go test ./... both exit 0
## Log
- 2026-08-10T22:32:26Z claimed by a-fixer-z57k0n
