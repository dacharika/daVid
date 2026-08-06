---
id: f-readme-documents-all-7-shipped-tools-incl-render-go-build-test-both-exit-0-from
kind: note
note_kind: finding
created: 2026-08-06T07:30:16Z
created_by: a-fixer-1w56em
about: "[[005]]"
severity: minor
---
# README documents all 7 shipped tools incl render; go build+test both exit 0 from clean checkout
README.md now covers describe/init/ingest/status/transcribe/timeline/render (matches cmd/cutvec/main.go dispatch and catalog.Default's 7 tools). One-sentence goal at README.md:3-8 matches the project Goal. Verified go build ./... and go test ./... both exit 0 from the README's Quick start (cmd/cutvec, axerror, catalog, project, worker ok; timeline no test files). Render section (README.md:104-130) pins output handle proj/cache/render-<hash>-<quality>.mp4, --proxy/--final tiers, cache-hit idempotency, and failure codes input.invalid/project.uninitialized/timeline.missing/worker.missing per renderCmd.
