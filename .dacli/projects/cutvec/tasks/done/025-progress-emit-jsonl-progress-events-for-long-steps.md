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
- [x] render, transcribe and speakers emit newline-delimited progress objects on stderr
- [x] stdout still carries exactly one JSON result, so the contract is unchanged
## Log
- 2026-08-10T17:39:00Z claimed by a-fixer-3z558s
- 2026-08-10T17:50:41Z a-fixer-3z558s: PR opened: https://github.com/mlnomadpy/cutvec/pull/28 (event 01KZPCMSPBDVRFYNCTAVHT54XT)
- 2026-08-10T17:56:26Z accepted by a-root
- 2026-08-10T17:56:26Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T17:56:26Z deliverable: dacli/025-progress-emit-jsonl-progress-events-for-long-steps is merged into trunk
- 2026-08-10T17:56:26Z completed by a-root
- 2026-08-10T17:56:30Z accepted by a-root
- 2026-08-10T17:56:30Z closed WITHOUT verification — no --verify command was given
- 2026-08-10T17:56:30Z deliverable: dacli/025-progress-emit-jsonl-progress-events-for-long-steps is merged into trunk
- 2026-08-10T17:56:30Z completed by a-root
