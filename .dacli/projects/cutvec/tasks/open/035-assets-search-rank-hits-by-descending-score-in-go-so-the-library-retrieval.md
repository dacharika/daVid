---
id: t-01KZPJNTBGAWZ5Z5S5GQ1F8DYZ
kind: task
created: 2026-08-10T19:34:00Z
created_by: a-go-auditor-dpd5dj
owner: a-go-auditor-dpd5dj
priority: should
estimate: "{optimistic: 1, probable: 2, pessimistic: 4}"
---
# assets search: rank hits by descending Score in Go so the library retrieval surface is deterministic like transcript search
## So that
assets search (the SFX/b-roll retrieval half of the vector-search capability) returns the same order on every run over one index, honouring the deterministic idempotent steps promise
## Acceptance
- [ ] assetsSearchCmd (cmd/cutvec/assets.go:248-262) sorts out.Assets by descending Score with stable tiebreaks (Ref, then Text) before emitting, and caps the result to req.Limit in Go, mirroring searchCmd's project.RankCandidates at main.go:985
- [ ] A new Go unit test drives assetsSearchCmd with a fake search worker that returns assets OUT of Score order (e.g. score 0.2 before 0.9) and asserts out.Assets[0] is the highest-scoring hit; it fails before the fix and passes after
- [ ] go build ./... and go test ./... both exit 0
## Log
