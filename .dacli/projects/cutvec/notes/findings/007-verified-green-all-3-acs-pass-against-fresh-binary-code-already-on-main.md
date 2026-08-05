---
id: f-007-verified-green-all-3-acs-pass-against-fresh-binary-code-already-on-main
kind: note
note_kind: finding
created: 2026-08-05T23:23:18Z
created_by: a-fixer-2wavrv
about: "[[007]]"
severity: major
---
# 007 verified green: all 3 ACs pass against fresh binary; code already on main
Built /tmp/cv007 from ./cmd/cutvec on branch dacli/007. 'git diff origin/main -- internal/catalog cmd/cutvec' is EMPTY (code landed via a merged PR). AC1: 'cutvec describe --json' returns a JSON array of exactly 5 tools, each with exactly {name,summary,inputSchema,outputSchema} (cmd/cutvec describeCmd + internal/catalog Default()). AC2: 'cutvec schema timeline' returns a draft-2020-12 doc ($schema=https://json-schema.org/draft/2020-12/schema) nesting top-level properties {input,output} via internal/catalog Tool.JSONSchema. AC3: 'go test ./internal/catalog' ok. Uncached 'go build ./...' exit 0 and 'go test -count=1 ./...' all packages ok (cmd/cutvec, axerror, catalog, project, worker; timeline no test files). No further code work or PR needed.
