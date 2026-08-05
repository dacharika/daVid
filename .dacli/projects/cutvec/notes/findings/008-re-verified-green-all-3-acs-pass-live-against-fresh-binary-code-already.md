---
id: f-008-re-verified-green-all-3-acs-pass-live-against-fresh-binary-code-already
kind: note
note_kind: finding
created: 2026-08-05T23:21:10Z
created_by: a-fixer-7db6yj
about: "[[008]]"
severity: major
---
# 008 re-verified green: all 3 ACs pass live against fresh binary; code already merged via PR #4
Built /tmp/cutvec008 from ./cmd/cutvec on branch dacli/008. git diff origin/main -- cmd/ internal/ is EMPTY (landed on main via merged PR #4, mergeCommit 3414c2c, mergedAt 2026-08-05T23:12:00Z, state MERGED). Live e2e (JSON on stdin): AC1 init writes <project>/project.json manifest {version:0,name,sources:[]}, returns created:true. AC2 status --json returns StateGraph {project,name,sources[],artifacts[]}; timeline artifact resolved status:pending from disk (project.json Status). AC3 ingest keys sources on sha256(bytes) (id=a948904f2f0f, hash=a948904f2f0f479b...): first ingest cached:false, re-ingest identical bytes cached:true with manifest holding exactly 1 source (idempotent) — asserted by cmd/cutvec/main_test.go:368 TestIngestCacheHitIdempotent + internal/project/project_test.go:22 TestIngestIsContentHashCacheHit. Failure rules pinned by TestStateModelFailureRules (main_test.go:451): ingest requires source (input.invalid), status requires init. Uncached go build ./... and go test ./... both exit 0 (cmd/cutvec, axerror, catalog, project, worker all ok). No further code work or PR needed.
