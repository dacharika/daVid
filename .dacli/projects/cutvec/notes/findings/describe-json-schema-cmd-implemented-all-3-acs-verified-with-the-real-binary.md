---
id: f-describe-json-schema-cmd-implemented-all-3-acs-verified-with-the-real-binary
kind: note
note_kind: finding
created: 2026-08-05T21:39:34Z
created_by: a-fixer-x4yg3c
about: "[[007]]"
severity: major
---
# describe --json + schema <cmd> implemented; all 3 ACs verified with the real binary and go test
AC1: 'cutvec describe --json' (cmd/cutvec/main.go:describeCmd) emits a JSON array of tools, each with name, summary, inputSchema, outputSchema (verified: /tmp/cutvec describe --json -> array). Bare 'describe' still emits the full {version,tools} catalog. AC2: 'cutvec schema <cmd>' (cmd/cutvec/main.go:schemaCmd -> catalog.Tool.JSONSchema in internal/catalog/catalog.go) returns a draft-2020-12 JSON Schema titled by the command, nesting input+output schemas (verified: /tmp/cutvec schema timeline). Unknown/empty/too-many-args map to stable axerror codes cli.unknown_command/input.empty/input.invalid, exit 1. AC3: 'go test ./internal/catalog' passes (added TestEveryToolIsFullyTyped, TestLookup, TestNames, TestJSONSchemaShape, TestJSONSchemaFillsMissingSchemas); 'go build ./...' and 'go test ./...' both exit 0; gofmt+go vet clean. Committed 3879b84 on branch dacli/007-....
