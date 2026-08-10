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
- [ ] ingest stores width, height, duration and fps on the source, via the ffmpeg worker
- [ ] status --json reports them; a re-ingest is still a cache hit
## Log
- 2026-08-10T16:36:50Z claimed by a-fixer-hx5gwf
- 2026-08-10T16:48:32Z a-fixer-hx5gwf: PR opened: https://github.com/mlnomadpy/cutvec/pull/25 (event 01KZP95S5CWB1B72B4F6ZAVRJK)
