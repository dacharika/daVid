---
id: t-01KZ9WMYYX9PJ87Z7MK17E7NSK
kind: task
created: 2026-08-05T21:18:10Z
created_by: a-root
owner: a-root
priority: should
estimate: "{optimistic: 5, probable: 8, pessimistic: 13}"
---
# grade + denoise + sfx timeline filters
## So that
clips get color grade, background-noise removal, and sfx via MLT filters
## Acceptance
- [x] timeline filter supports a grade, a denoise (rnnoise/afftdn), and sfx mix; each is an MLT filter node
## Log
- 2026-08-10T15:05:24Z accepted by a-root
- 2026-08-10T15:05:24Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T15:05:24Z deliverable: no dacli/020-grade-denoise-sfx-timeline-filters branch — nothing to check against trunk
- 2026-08-10T15:05:24Z completed by a-root
