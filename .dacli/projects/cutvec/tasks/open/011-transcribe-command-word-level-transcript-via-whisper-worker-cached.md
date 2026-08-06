---
id: t-01KZ9WMYR510C9KDHBYXFZ0AWB
kind: task
created: 2026-08-05T21:18:10Z
created_by: a-root
owner: a-root
priority: must
estimate: "{optimistic: 5, probable: 8, pessimistic: 13}"
---
# transcribe command: word-level transcript via whisper worker, cached
## So that
an agent gets timecoded words to search and to build captions
## Acceptance
- [ ] cutvec transcribe <project> --json returns a handle; transcript has word-level start/end
- [ ] result is cached and idempotent
## Log
- 2026-08-05T23:41:02Z claimed by a-fixer-sv7wx8
