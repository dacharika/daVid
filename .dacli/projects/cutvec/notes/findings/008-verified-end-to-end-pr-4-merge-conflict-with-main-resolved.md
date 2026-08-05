---
id: f-008-verified-end-to-end-pr-4-merge-conflict-with-main-resolved
kind: note
note_kind: finding
created: 2026-08-05T23:12:06Z
created_by: a-fixer-79gdr2
about: "[[008]]"
severity: major
---
# 008 verified end-to-end; PR #4 merge conflict with main resolved
PR #4 (branch dacli/008) was CONFLICTING against main because origin/main advanced with PR #3 (task 007: describe --json + schema) touching cmd/cutvec/main.go's dispatch and internal/catalog/catalog.go. Merged origin/main into the branch (commit 3dfd4f4); sole content conflict was the describe-case in cmd/cutvec/main.go:35-49 run() dispatch — resolved by keeping BOTH task 007's 'describe'->describeCmd + 'schema'->schemaCmd AND task 008's 'init'/'ingest'/'status' cases. After merge: gofmt clean, go vet clean, 'go build ./...' and 'go test ./...' both exit 0. PR #4 is now MERGEABLE; auto-merge (squash) queued; CI 'Go build and test' running (run 31055494336). All 3 ACs verified with the real binary: (1) init writes project.json {version,name,sources} created:true; (2) status --json returns {project,name,sources[],artifacts[]} with timeline pending->done from disk; (3) re-ingest of identical bytes returns cached:false then cached:true keyed on sha256, manifest holds exactly 1 source (idempotent), asserted by TestIngestCacheHitIdempotent + TestIngestIsContentHashCacheHit.
