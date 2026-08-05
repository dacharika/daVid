---
id: f-007-re-verified-green-all-3-acs-pass-against-fresh-binary-work-already-on-main
kind: note
note_kind: finding
created: 2026-08-05T23:19:15Z
created_by: a-fixer-vx7753
about: "[[007]]"
severity: major
---
# 007 re-verified green: all 3 ACs pass against fresh binary; work already on main via PR #3
Built /tmp/cv007 from ./cmd/cutvec on branch dacli/007. git diff origin/main -- internal/catalog cmd/cutvec is EMPTY (landed via PR #3). AC1: describe --json returns JSON array of 5 tools, each with exactly {name,summary,inputSchema,outputSchema} (cmd/cutvec/main.go describeCmd + internal/catalog/catalog.go Default()). AC2: schema timeline returns draft-2020-12 doc ($schema=https://json-schema.org/draft/2020-12/schema, title=timeline) nesting properties {input,output} (internal/catalog Tool.JSONSchema); unknown cmd -> {code:cli.unknown_command,retryable:false} exit 1. AC3: go test ./internal/catalog ok; full go build ./... and go test ./... both exit 0 (cmd/cutvec, axerror, catalog, project, worker all ok).
