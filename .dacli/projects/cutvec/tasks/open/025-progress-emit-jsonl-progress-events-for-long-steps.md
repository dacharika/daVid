---
id: t-01KZP3BTB7CRG04PV11D9QA7JN
kind: task
created: 2026-08-10T15:06:23Z
created_by: a-root
owner: a-root
priority: should
estimate: "{optimistic: 3, probable: 5, pessimistic: 8}"
---
# progress: emit JSONL progress events for long steps
## So that
an agent can tell a slow render from a hung one
## Acceptance
- [ ] render, transcribe and speakers emit newline-delimited progress objects on stderr
- [ ] stdout still carries exactly one JSON result, so the contract is unchanged
## Log
- 2026-08-10T17:39:00Z claimed by a-fixer-3z558s
- 2026-08-10T17:50:41Z a-fixer-3z558s: PR opened: https://github.com/mlnomadpy/cutvec/pull/28 (event 01KZPCMSPBDVRFYNCTAVHT54XT)
