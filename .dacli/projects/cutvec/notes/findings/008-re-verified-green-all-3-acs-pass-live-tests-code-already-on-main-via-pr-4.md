---
id: f-008-re-verified-green-all-3-acs-pass-live-tests-code-already-on-main-via-pr-4
kind: note
note_kind: finding
created: 2026-08-05T23:23:40Z
created_by: a-fixer-zf590g
about: "[[008]]"
severity: major
---
# 008 re-verified green: all 3 ACs pass live + tests; code already on main via PR #4
In isolated worktree on branch dacli/008. 'git diff origin/main -- cmd/ internal/' is EMPTY (code landed on main via merged PR #4, commit 3414c2c). UNCACHED 'go build ./...' exit 0; 'go test -count=1 ./...' ALL ok (cmd/cutvec, axerror, catalog, project, worker; timeline no test files). Live e2e against fresh binary: AC1 init writes demo/project.json manifest {version:0,name,sources:[]}, returns created:true (TestInitCreatesManifest cmd/cutvec/main_test.go:244, TestInitIdempotent :279). AC2 status --json returns StateGraph{project,name,sources[],artifacts[]}; timeline artifact resolved status:pending from disk (TestStatusStateGraph main_test.go:305, TestStatusReflectsArtifacts internal/project/project_test.go:89). AC3 ingest keys sources on sha256(bytes) (id=d667df3e603b, hash=d667df3e603bd8c3...); first ingest cached:false, re-ingest identical bytes cached:true with manifest holding exactly 1 source (idempotent) (TestIngestCacheHitIdempotent main_test.go:368, TestIngestIsContentHashCacheHit project_test.go:22). Failure rules pinned by TestStateModelFailureRules main_test.go:451. No further code work or PR needed; task boxes just needed checking.
