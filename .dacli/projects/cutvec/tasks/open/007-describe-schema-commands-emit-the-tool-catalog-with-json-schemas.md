---
id: t-01KZ9WMYMZ2CKRTPN66C94SD93
kind: task
created: 2026-08-05T21:18:10Z
created_by: a-root
owner: a-root
priority: must
estimate: "{optimistic: 3, probable: 5, pessimistic: 8}"
---
# describe + schema commands emit the tool catalog with JSON Schemas
## So that
an agent can discover every command and its typed IO, and the same catalog backs the MCP server
## Acceptance
- [ ] cutvec describe --json returns an array of tools each with name, summary, input schema, output schema
- [ ] cutvec schema <cmd> returns the JSON Schema for that command
- [ ] go test ./internal/catalog passes
## Log
