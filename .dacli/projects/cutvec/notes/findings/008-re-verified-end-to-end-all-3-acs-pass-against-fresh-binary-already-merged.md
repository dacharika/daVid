---
id: f-008-re-verified-end-to-end-all-3-acs-pass-against-fresh-binary-already-merged
kind: note
note_kind: finding
created: 2026-08-05T23:15:03Z
created_by: a-fixer-4djp4w
about: "[[008]]"
severity: major
---
# 008 re-verified end-to-end: all 3 ACs pass against fresh binary; already merged via PR #4
Built /tmp/cutvec008 from ./cmd/cutvec on branch dacli/008 (work merged to main at commit 3414c2c via PR #4). AC1: 'init' with {project,name} writes <project>/project.json {version:"0",name,sources:[]} and returns created:true (cmd/cutvec/main.go initCmd + internal/project Project.Init). AC2: 'status --json' with {project} returns StateGraph {project,name,sources[],artifacts[]}; timeline artifact resolved status:pending from disk (internal/project project.go Status). AC3: 'ingest' keys each source on sha256(bytes) (id=a948904f2f0f, hash=a948904f2f0f479b...); first ingest cached:false, re-ingest of identical bytes cached:true with manifest holding exactly 1 source (idempotent), asserted by cmd/cutvec/main_test.go TestIngestCacheHitIdempotent + internal/project/project_test.go TestIngestIsContentHashCacheHit. Failure rules pinned by TestStateModelFailureRules. go build ./..., go vet ./..., gofmt -l . (clean), go test ./... all exit 0. Checking acceptance boxes 1-3 and marking done.
