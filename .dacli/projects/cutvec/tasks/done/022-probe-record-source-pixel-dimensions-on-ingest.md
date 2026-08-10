---
id: t-01KZP3BT8CZXT58J5WJN5B91W5
kind: task
created: 2026-08-10T15:06:23Z
created_by: a-root
owner: a-root
priority: must
estimate: "{optimistic: 2, probable: 4, pessimistic: 6}"
---
# probe: record source pixel dimensions on ingest
## So that
framing can compute a real crop window instead of assuming one
## Acceptance
- [x] ingest stores width, height, duration and fps on the source, via the ffmpeg worker
- [x] status --json reports them; a re-ingest is still a cache hit
## Log
- 2026-08-10T16:36:50Z claimed by a-fixer-hx5gwf
- 2026-08-10T16:48:32Z a-fixer-hx5gwf: PR opened: https://github.com/mlnomadpy/cutvec/pull/25 (event 01KZP95S5CWB1B72B4F6ZAVRJK)
- 2026-08-10T16:55:02Z accepted by a-root
- 2026-08-10T16:55:02Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T16:55:02Z deliverable: dacli/022-probe-record-source-pixel-dimensions-on-ingest is merged into trunk
- 2026-08-10T16:55:02Z completed by a-root
- 2026-08-10T16:55:05Z accepted by a-root
- 2026-08-10T16:55:05Z closed WITHOUT verification — no --verify command was given
- 2026-08-10T16:55:05Z deliverable: dacli/022-probe-record-source-pixel-dimensions-on-ingest is merged into trunk
- 2026-08-10T16:55:05Z completed by a-root
- 2026-08-10T17:20:47Z accepted by a-root
- 2026-08-10T17:20:47Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T17:20:47Z deliverable: no dacli/022-probe-record-source-pixel-dimensions-on-ingest branch — nothing to check against trunk
- 2026-08-10T17:20:47Z completed by a-root
- 2026-08-10T17:35:39Z accepted by a-root
- 2026-08-10T17:35:39Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T17:35:39Z deliverable: no dacli/022-probe-record-source-pixel-dimensions-on-ingest branch — nothing to check against trunk
- 2026-08-10T17:35:39Z completed by a-root
- 2026-08-10T17:56:25Z accepted by a-root
- 2026-08-10T17:56:25Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T17:56:25Z deliverable: no dacli/022-probe-record-source-pixel-dimensions-on-ingest branch — nothing to check against trunk
- 2026-08-10T17:56:25Z completed by a-root
- 2026-08-10T18:10:48Z accepted by a-root
- 2026-08-10T18:10:48Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T18:10:48Z deliverable: no dacli/022-probe-record-source-pixel-dimensions-on-ingest branch — nothing to check against trunk
- 2026-08-10T18:10:48Z completed by a-root
- 2026-08-10T18:19:16Z accepted by a-root
- 2026-08-10T18:19:16Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T18:19:16Z deliverable: no dacli/022-probe-record-source-pixel-dimensions-on-ingest branch — nothing to check against trunk
- 2026-08-10T18:19:16Z completed by a-root
- 2026-08-10T18:25:58Z accepted by a-root
- 2026-08-10T18:25:58Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T18:25:58Z deliverable: no dacli/022-probe-record-source-pixel-dimensions-on-ingest branch — nothing to check against trunk
- 2026-08-10T18:25:58Z completed by a-root
- 2026-08-10T18:36:08Z accepted by a-root
- 2026-08-10T18:36:08Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T18:36:08Z deliverable: no dacli/022-probe-record-source-pixel-dimensions-on-ingest branch — nothing to check against trunk
- 2026-08-10T18:36:08Z completed by a-root
- 2026-08-10T18:42:37Z accepted by a-root
- 2026-08-10T18:42:37Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T18:42:37Z deliverable: no dacli/022-probe-record-source-pixel-dimensions-on-ingest branch — nothing to check against trunk
- 2026-08-10T18:42:37Z completed by a-root
