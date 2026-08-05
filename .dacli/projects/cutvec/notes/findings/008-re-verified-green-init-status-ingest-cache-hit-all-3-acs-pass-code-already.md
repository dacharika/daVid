---
id: f-008-re-verified-green-init-status-ingest-cache-hit-all-3-acs-pass-code-already
kind: note
note_kind: finding
created: 2026-08-05T23:19:11Z
created_by: a-fixer-z5abvh
about: "[[008]]"
severity: major
---
# 008 re-verified green: init/status/ingest cache-hit all 3 ACs pass; code already on main via PR #4
Re-verified in worktree on branch dacli/008. 'git diff origin/main -- cmd/ internal/' is EMPTY — task 008 code landed on main via merged PR #4 (commit 3414c2c). Uncached 'go test -count=1 ./cmd/cutvec ./internal/project' both ok. AC1: init writes <project>/project.json manifest (TestInitCreatesManifest main_test.go:244, TestInitIdempotent :279). AC2: status --json returns StateGraph{project,name,sources[],artifacts[]} with artifacts resolved done/pending from disk (TestStatusStateGraph main_test.go:305, TestStatusReflectsArtifacts project_test.go:89). AC3: ingest keys sources on sha256(bytes) so re-ingest of identical bytes is cached:true with exactly 1 source in manifest — idempotent (TestIngestCacheHitIdempotent main_test.go:368, TestIngestIsContentHashCacheHit project_test.go:22). Failure rules pinned by TestStateModelFailureRules main_test.go:451. go build ./... and go test ./... both exit 0. Owner may check boxes 1-3 and mark 008 done; no further code or PR needed.
