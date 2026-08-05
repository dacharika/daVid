---
id: t-01KZ9WMYTERKPZHFHV79QMQS37
kind: task
created: 2026-08-05T21:18:10Z
created_by: a-root
owner: a-root
priority: must
estimate: "{optimistic: 3, probable: 5, pessimistic: 8}"
---
# search command: semantic query returns ranked timecoded candidates
## So that
an agent asks for a moment by meaning and gets timecodes + scores
## Acceptance
- [ ] cutvec search <project> '<query>' --json returns ranked {start,end,text,score}
- [ ] results are deterministic for a fixed query+index
## Log
