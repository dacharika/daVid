---
id: f-008-re-verified-end-to-end-init-status-ingest-acs-all-pass-pr-4-merged-to-main
kind: note
note_kind: finding
created: 2026-08-05T23:13:44Z
created_by: a-fixer-782fh2
about: "[[008]]"
severity: major
---
# 008 re-verified end-to-end: init/status/ingest ACs all pass, PR #4 merged to main
Independently re-verified all 3 ACs against a freshly-built binary (go build -o $TMP ./cmd/cutvec). AC1: 'init' with {project,name} on stdin writes <project>/project.json {version,name,sources:[]} and returns created:true (cmd/cutvec/main.go + internal/project.Project.Init). AC2: 'status --json' with {project} returns StateGraph {project,name,sources[],artifacts[]}; the timeline artifact is resolved status:pending from disk (project.go Status). AC3: 'ingest' keys each source on sha256(bytes); first ingest cached:false, re-ingest of identical bytes cached:true with the manifest holding exactly 1 source (idempotent), asserted by cmd/cutvec/main_test.go TestIngestCacheHitIdempotent and internal/project/project_test.go TestIngestIsContentHashCacheHit. Failure rules pinned by TestStateModelFailureRules (input.empty/input.invalid/project.uninitialized/source.missing, structured axerror, exit 1). 'go build ./...' and 'go test ./...' both exit 0; work already landed on main via merged PR #4 (commit 3414c2c). Task boxes were still unchecked; checking them now.
