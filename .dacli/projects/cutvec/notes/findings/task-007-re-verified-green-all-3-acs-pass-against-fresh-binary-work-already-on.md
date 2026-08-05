---
id: f-task-007-re-verified-green-all-3-acs-pass-against-fresh-binary-work-already-on
kind: note
note_kind: finding
created: 2026-08-05T23:17:41Z
created_by: a-fixer-mk6ege
about: "[[007]]"
severity: major
---
# Task 007 re-verified green: all 3 ACs pass against fresh binary, work already on main
Built /tmp/cv007 from ./cmd/cutvec on branch dacli/007. 'git diff origin/main -- internal/catalog cmd/cutvec' is EMPTY (landed via PR #3). AC1: 'describe --json' returns a JSON array of 5 tools, each with exactly {name,summary,inputSchema,outputSchema} (cmd/cutvec/main.go describeCmd + internal/catalog/catalog.go Default()). AC2: 'schema timeline' returns a draft-2020-12 doc ($schema=https://json-schema.org/draft/2020-12/schema) nesting input+output; unknown cmd -> structured axerror {code:cli.unknown_command,retryable:false} exit 1 (internal/catalog Tool.JSONSchema). AC3: 'go test ./internal/catalog' ok. Full 'go build ./...' and 'go test ./...' both exit 0.
