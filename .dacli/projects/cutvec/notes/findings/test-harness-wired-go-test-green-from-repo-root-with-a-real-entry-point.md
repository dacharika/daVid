---
id: f-test-harness-wired-go-test-green-from-repo-root-with-a-real-entry-point
kind: note
note_kind: finding
created: 2026-08-05T14:35:11Z
created_by: a-fixer-br37c8
about: "[[002]]"
severity: moderate
---
# Test harness wired: go test ./... green from repo root with a real entry-point assertion; deliberate-failure exit=1 observed then removed
cmd/cutvec/main_test.go:12 TestRunDispatch exercises run() for describe/usage/help (nil) and mcp/unknown (axerror codes mcp.unimplemented, cli.unknown_command) via errors.As on *axerror.Error. 'go test ./...' exits 0 (cmd/cutvec, internal/axerror, internal/catalog pass; project/timeline/worker have no tests yet). Deliberate red: a temporary TestHarnessCatchesFailure asserting 1+1==3 was added, 'go test ./...' observed exit=1 with '--- FAIL: TestHarnessCatchesFailure', then the file was removed and the suite returned exit=0. README.md:16 already documents the green run from a clean checkout.
