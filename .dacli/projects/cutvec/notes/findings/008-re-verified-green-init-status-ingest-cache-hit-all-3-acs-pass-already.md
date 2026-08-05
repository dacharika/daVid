---
id: f-008-re-verified-green-init-status-ingest-cache-hit-all-3-acs-pass-already
kind: note
note_kind: finding
created: 2026-08-05T23:16:17Z
created_by: a-fixer-vk56jt
about: "[[008]]"
severity: major
---
# 008 re-verified green: init/status/ingest cache-hit all 3 ACs pass, already merged via PR #4
Verified against fresh binary (go build -o /tmp/cutvec008 ./cmd/cutvec) in worktree on branch dacli/008. git diff origin/main is ZERO code (only .dacli workspace files) — task 008 landed on main via merged PR #4 (commit 3414c2c). AC1: init {project,name} writes <project>/project.json {version:"0",name,sources:[]} returning created:true (cmd/cutvec/main.go initCmd + internal/project Project.Init). AC2: status --json {project} returns StateGraph {project,name,sources[],artifacts[]}; timeline artifact resolved status:pending from disk (internal/project/project.go Status). AC3: ingest keys sources on sha256(bytes) (id=2cf24dba5fb0, hash=2cf24dba5fb0a30e...); first ingest cached:false, re-ingest identical bytes cached:true with manifest holding exactly 1 source (idempotent), asserted by cmd/cutvec/main_test.go:368 TestIngestCacheHitIdempotent + internal/project/project_test.go:22 TestIngestIsContentHashCacheHit. Failure rules pinned by TestStateModelFailureRules (main_test.go:451). go build ./... and go test ./... both exit 0 (cmd/cutvec, axerror, catalog, project, worker all ok).
