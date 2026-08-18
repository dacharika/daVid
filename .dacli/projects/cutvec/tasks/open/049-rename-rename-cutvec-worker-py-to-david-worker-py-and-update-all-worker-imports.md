---
id: t-01M0ADSA6GSW2AE5RST687S1HZ
kind: task
created: 2026-08-18T12:33:20Z
created_by: a-root
owner: a-root
priority: must
estimate: "{optimistic: 1, probable: 2, pessimistic: 3}"
depends_on: "[047]"
---
# rename: rename cutvec_worker.py to david_worker.py and update all worker imports
## So that
the Python worker layer uses the canonical daVid identity and agents see david_worker as the base module
## Acceptance
- [ ] workers/cutvec_worker.py is gone; workers/david_worker.py contains its content
- [ ] every worker that imported cutvec_worker now imports david_worker (grep for cutvec_worker in workers/ returns no hits)
- [ ] ruff check workers/ and ruff format --check workers/ both exit 0
- [ ] python3 -m compileall -q workers/ exits 0
- [ ] go build ./... and go test ./... both exit 0
## Log
- 2026-08-18T12:34:31Z claimed by a-ctky0m2cwb
