---
id: t-01KZPJNTBGAWZ5Z5S5GQ1F8DYZ
kind: task
created: 2026-08-10T19:34:00Z
created_by: a-go-auditor-dpd5dj
owner: a-root
priority: should
estimate: "{optimistic: 1, probable: 2, pessimistic: 4}"
---
# assets search: rank hits by descending Score in Go so the library retrieval surface is deterministic like transcript search
## So that
assets search (the SFX/b-roll retrieval half of the vector-search capability) returns the same order on every run over one index, honouring the deterministic idempotent steps promise
## Acceptance
- [x] assetsSearchCmd (cmd/cutvec/assets.go:248-262) sorts out.Assets by descending Score with stable tiebreaks (Ref, then Text) before emitting, and caps the result to req.Limit in Go, mirroring searchCmd's project.RankCandidates at main.go:985
- [x] A new Go unit test drives assetsSearchCmd with a fake search worker that returns assets OUT of Score order (e.g. score 0.2 before 0.9) and asserts out.Assets[0] is the highest-scoring hit; it fails before the fix and passes after
- [x] go build ./... and go test ./... both exit 0
## Log
- 2026-08-10T20:57:57Z adopted by a-root (owner a-go-auditor-dpd5dj orphaned)
- 2026-08-10T22:18:34Z claimed by a-fixer-dkm65b
- 2026-08-10T22:21:49Z a-fixer-dkm65b: PR opened: https://github.com/mlnomadpy/cutvec/pull/41 (event 01KZPW8GZTSS78JX22W7GDQY81)
- 2026-08-10T22:32:26Z accepted by a-root (applied 1 proposal(s))
- 2026-08-10T22:32:26Z closed WITHOUT verification — no --verify command was given
- 2026-08-10T22:32:26Z deliverable: dacli/035-assets-search-rank-hits-by-descending-score-in-go-so-the-library-retrieval is merged into main
- 2026-08-10T22:32:26Z completed by a-root
- 2026-08-11T05:04:31Z accepted by a-root
- 2026-08-11T05:04:31Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-11T05:04:31Z deliverable: no dacli/035-assets-search-rank-hits-by-descending-score-in-go-so-the-library-retrieval branch — nothing to check against main
- 2026-08-11T05:04:31Z completed by a-root
- 2026-08-11T08:12:10Z accepted by a-root
- 2026-08-11T08:12:10Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-11T08:12:10Z deliverable: no dacli/035-assets-search-rank-hits-by-descending-score-in-go-so-the-library-retrieval branch — nothing to check against main
- 2026-08-11T08:12:10Z completed by a-root
