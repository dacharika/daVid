---
id: f-008-independently-re-verified-green-all-3-acs-pass-uncached-code-merged-via-pr-4
kind: note
note_kind: finding
created: 2026-08-05T23:22:15Z
created_by: a-fixer-bpbjbx
about: "[[008]]"
severity: major
---
# 008 independently re-verified green: all 3 ACs pass uncached, code merged via PR #4
Branch dacli/008 in isolated worktree. 'git diff origin/main -- cmd/ internal/' is EMPTY (landed on main via merged PR #4, commit 3414c2c). Uncached 'go build ./...' exit 0 and 'go test -count=1 ./...' ALL ok (cmd/cutvec, axerror, catalog, project, worker; timeline no test files). AC1 init writes <project>/project.json manifest: TestInitCreatesManifest main_test.go:244 + TestInitIdempotent :279. AC2 status --json returns StateGraph{project,name,sources[],artifacts[]} resolved from disk: TestStatusStateGraph main_test.go:305 + TestStatusReflectsArtifacts project_test.go:89. AC3 ingest keys sources on sha256(bytes), re-ingest identical bytes cached:true with exactly 1 source (idempotent): TestIngestCacheHitIdempotent main_test.go:368 + TestIngestIsContentHashCacheHit project_test.go:22. Failure rules pinned by TestStateModelFailureRules main_test.go:451. No further code work or PR needed.
