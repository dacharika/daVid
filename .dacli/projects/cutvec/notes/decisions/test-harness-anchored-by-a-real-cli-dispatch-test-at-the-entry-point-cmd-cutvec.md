---
id: d-test-harness-anchored-by-a-real-cli-dispatch-test-at-the-entry-point-cmd-cutvec
kind: note
note_kind: decision
created: 2026-08-05T14:34:05Z
created_by: a-fixer-br37c8
about: "[[002]]"
---
# Test harness anchored by a real CLI-dispatch test at the entry point (cmd/cutvec)
## Chose
Test harness anchored by a real CLI-dispatch test at the entry point (cmd/cutvec)
## Rejected
Relying only on the existing internal/axerror + internal/catalog unit tests to satisfy 'at least one real assertion'
## Because
The project goal is that the first feature slice runs from its user-visible entry point (the CLI) to a returned result; a test on run() in cmd/cutvec exercises that surface, so a green go test ./... proves the entry point dispatches, not just that leaf packages work. cmd/cutvec previously reported [no test files].
