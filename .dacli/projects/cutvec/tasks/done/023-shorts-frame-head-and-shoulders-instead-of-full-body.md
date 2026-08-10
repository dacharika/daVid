---
id: t-01KZP3BT9BKR1P96JCFPY6NR6J
kind: task
created: 2026-08-10T15:06:23Z
created_by: a-root
owner: a-root
priority: must
estimate: "{optimistic: 3, probable: 5, pessimistic: 8}"
depends_on: "[022]"
---
# shorts: frame head-and-shoulders instead of full body
## So that
a vertical short is watchable on a phone, which is the whole point of the format
## Acceptance
- [x] the crop is an explicit window computed from source dimensions and the detected seat, not center=1
- [x] a rendered short visibly frames head-and-shoulders; verified from a frame, not from the XML
## Log
- 2026-08-10T16:55:05Z claimed by a-fixer-pwpjwm
- 2026-08-10T17:14:19Z a-fixer-pwpjwm: PR opened: https://github.com/mlnomadpy/cutvec/pull/26 (event 01KZPAMCMFKFSP33H0AVTG5B0V)
- 2026-08-10T17:20:47Z accepted by a-root
- 2026-08-10T17:20:47Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T17:20:47Z deliverable: dacli/023-shorts-frame-head-and-shoulders-instead-of-full-body is merged into trunk
- 2026-08-10T17:20:47Z completed by a-root
- 2026-08-10T17:20:50Z accepted by a-root
- 2026-08-10T17:20:50Z closed WITHOUT verification — no --verify command was given
- 2026-08-10T17:20:50Z deliverable: dacli/023-shorts-frame-head-and-shoulders-instead-of-full-body is merged into trunk
- 2026-08-10T17:20:50Z completed by a-root
- 2026-08-10T17:35:40Z accepted by a-root
- 2026-08-10T17:35:40Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T17:35:40Z deliverable: no dacli/023-shorts-frame-head-and-shoulders-instead-of-full-body branch — nothing to check against trunk
- 2026-08-10T17:35:40Z completed by a-root
- 2026-08-10T17:56:25Z accepted by a-root
- 2026-08-10T17:56:25Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T17:56:25Z deliverable: no dacli/023-shorts-frame-head-and-shoulders-instead-of-full-body branch — nothing to check against trunk
- 2026-08-10T17:56:25Z completed by a-root
