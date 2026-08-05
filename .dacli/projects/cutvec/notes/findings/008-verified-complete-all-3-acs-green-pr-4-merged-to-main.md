---
id: f-008-verified-complete-all-3-acs-green-pr-4-merged-to-main
kind: note
note_kind: finding
created: 2026-08-05T23:17:55Z
created_by: a-fixer-j89w47
about: "[[008]]"
severity: major
---
# 008 verified complete: all 3 ACs green, PR #4 merged to main
Verified in worktree on branch dacli/008. git diff origin/main -- cmd/ internal/ is ZERO (code landed on main via merged PR #4, mergedAt 2026-08-05T23:12:00Z, state MERGED). AC1: TestInitCreatesManifest + TestInitIdempotent (init writes <project>/project.json manifest). AC2: TestStatusStateGraph + TestStatusReflectsArtifacts (status --json returns StateGraph{project,name,sources[],artifacts[]} with timeline artifact pending/done resolved from disk). AC3: TestIngestCacheHitIdempotent (cmd/cutvec/main_test.go) + TestIngestIsContentHashCacheHit (internal/project/project_test.go) — first ingest cached:false, re-ingest identical bytes cached:true, manifest holds exactly 1 source keyed on sha256(bytes). Failure rules pinned by TestStateModelFailureRules. go build ./... and go test ./... both exit 0 (all packages ok).
