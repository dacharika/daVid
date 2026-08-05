---
id: f-task-007-complete-all-3-acs-verified-green-pr-3-merged-to-main
kind: note
note_kind: finding
created: 2026-08-05T23:15:13Z
created_by: a-fixer-qzvy6k
about: "[[007]]"
severity: major
---
# Task 007 complete: all 3 ACs verified green, PR #3 MERGED to main
Re-verified against a fresh binary (go build -o /tmp/cutvec007 ./cmd/cutvec) in worktree on branch dacli/007. git diff origin/main -- internal/catalog cmd/cutvec shows ZERO diff (work already landed via PR #3, mergedAt 2026-08-05T21:48:06Z, state MERGED).

AC1 (describe --json array of typed tools): '/tmp/cutvec007 describe --json' returns a JSON array of 5 tools [describe,init,ingest,status,timeline], each with exactly keys {name,summary,inputSchema,outputSchema}. Impl: cmd/cutvec/main.go describeCmd emits the bare tools array; internal/catalog/catalog.go Default() seeds them.

AC2 (schema <cmd> returns JSON Schema): '/tmp/cutvec007 schema timeline' returns a draft-2020-12 doc (=https://json-schema.org/draft/2020-12/schema, title=timeline) nesting properties {input,output}. Impl: internal/catalog Tool.JSONSchema. Unknown cmd -> structured axerror {code:cli.unknown_command,retryable:false} exit 1.

AC3 (go test ./internal/catalog passes): PASS, 7 tests (TestDefaultSeedsDescribe, TestDefaultMarshalsToJSON, TestEveryToolIsFullyTyped, TestLookup, TestNames, TestJSONSchemaShape, TestJSONSchemaFillsMissingSchemas). Full 'go build ./...' and 'go test ./...' both exit 0.

Owner (a-root) can check acceptance boxes 1-3 and mark task 007 done; no further code work or PR is needed.
