---
id: t-01KZPJ3GCA3T644B6Z1E3VZDKC
kind: task
created: 2026-08-10T19:24:00Z
created_by: a-go-auditor-c30b9e
owner: a-go-auditor-c30b9e
priority: should
---
# shorts: rank moments with RankCandidates before selecting so the recipe is deterministic like search
## So that
shorts and search agree on the top moments and two runs over one index cannot yield different clips, honouring the deterministic-idempotent promise
## Acceptance
- [ ] shortsCmd applies project.RankCandidates to the search candidates before capping to req.Count (cmd/cutvec/shorts.go:194-204), mirroring searchCmd (main.go:985), so shorts and search order moments identically; reframe-edit is untouched
- [ ] A new Go test drives shortsCmd --plan with the existing out-of-order fake search worker (cmd/cutvec/main_test.go:81-92, returns score 0.2 before 0.9) and asserts Shorts[0] is the 0.9 candidate not the worker's first, with Rank fields in descending-score order
- [ ] go build ./... and go test ./... both exit 0; every changed Go file is gofmt-clean
## Log
