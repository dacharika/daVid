---
id: f-task-007-re-verified-green-all-3-acs-pass-against-fresh-binary-work-merged-via
kind: note
note_kind: finding
created: 2026-08-05T23:16:08Z
created_by: a-fixer-9efq2w
about: "[[007]]"
severity: major
---
# Task 007 re-verified green: all 3 ACs pass against fresh binary, work merged via PR #3
Built /tmp/cutvec007 from ./cmd/cutvec. git diff origin/main -- internal/catalog cmd/cutvec is EMPTY (landed on main via PR #3). AC1: 'describe --json' returns a JSON array of tools each with exactly {name,summary,inputSchema,outputSchema} (cmd/cutvec/main.go describeCmd + internal/catalog/catalog.go Default()). AC2: 'schema timeline' returns a draft-2020-12 doc ($schema=https://json-schema.org/draft/2020-12/schema) nesting properties {input,output} (internal/catalog Tool.JSONSchema); unknown cmd -> structured axerror {code:cli.unknown_command,retryable:false} exit 1. AC3: 'go test ./internal/catalog' ok; full 'go build ./...' and 'go test ./...' both exit 0.
