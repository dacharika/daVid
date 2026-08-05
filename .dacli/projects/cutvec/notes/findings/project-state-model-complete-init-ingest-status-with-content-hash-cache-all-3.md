---
id: f-project-state-model-complete-init-ingest-status-with-content-hash-cache-all-3
kind: note
note_kind: finding
created: 2026-08-05T23:08:26Z
created_by: a-fixer-kthys7
about: "[[008]]"
severity: major
---
# Project state model complete: init/ingest/status with content-hash cache; all 3 acceptance criteria met and tested
AC1 (init creates project.json manifest): 'cutvec init' reads {project,name} on stdin, creates <project>/project.json via internal/project.Project.Init (project.go), returns {project,manifest,name,created}. Idempotent — re-init returns created:false without clobbering. Tests: cmd/cutvec/main_test.go TestInitCreatesManifest, TestInitIdempotent. Verified with real binary.

AC2 (status --json returns state graph): 'cutvec status --json' returns internal/project.StateGraph {project,name,sources[],artifacts[]}, each artifact marked done/pending from disk (project.go Status/artifact). Project from stdin {project} or defaults to cwd. Test: TestStatusStateGraph asserts the ingested source appears and the timeline artifact flips pending->done once built.

AC3 (re-running a completed step is a cache hit, asserted by a test): 'cutvec ingest' keys each source on sha256(file bytes) (project.go HashFile/Ingest); a cache entry under <project>/cache/<hash>.json makes a re-ingest of unchanged bytes return cached:true with NO manifest mutation. Tests: cmd/cutvec/main_test.go TestIngestCacheHitIdempotent (cached:false then true; manifest holds exactly 1 source after re-ingest) and internal/project/project_test.go TestIngestIsContentHashCacheHit. Verified with real binary: second ingest of same file -> cached:true.

Tools registered in internal/catalog/catalog.go with input/output JSON Schemas (backs describe + MCP). --plan dry-run on init/ingest (TestIngestPlanWritesNothing). Failure rules pinned in TestStateModelFailureRules: input.empty / input.invalid / project.uninitialized / source.missing (structured axerror, exit 1). gofmt + go vet clean; 'go build ./...' and 'go test ./...' both exit 0. Commit 2dc51b7.
