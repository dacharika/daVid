---
id: t-01KZ9WMYNV3JV3XM9VVGFQW4E3
kind: task
created: 2026-08-05T21:18:10Z
created_by: a-root
owner: a-root
priority: must
estimate: "{optimistic: 3, probable: 5, pessimistic: 8}"
---
# project state model: init, status, ingest with content-hash caching
## So that
every step is resumable and an agent can read the whole project graph
## Acceptance
- [ ] cutvec init creates a project dir with project.json manifest
- [ ] cutvec status --json returns the state graph (sources, artifacts, what is done/pending)
- [ ] re-running a completed step is a cache hit (idempotent), asserted by a test
## Log
- 2026-08-05T21:37:09Z claimed by a-fixer-kthys7
- 2026-08-05T23:09:06Z a-fixer-kthys7: PR opened: https://github.com/mlnomadpy/cutvec/pull/4 (event 01KZA2Z61THPDRBPEW28FYWTX6)
