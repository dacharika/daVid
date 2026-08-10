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
- [x] cutvec describe --json returns an array of tools each with name, summary, input schema, output schema
- [x] cutvec schema <cmd> returns the JSON Schema for that command
- [x] go test ./internal/catalog passes
## Log
- 2026-08-05T21:37:09Z claimed by a-fixer-x4yg3c
- 2026-08-05T23:09:06Z a-fixer-x4yg3c: PR opened: https://github.com/mlnomadpy/cutvec/pull/3 (event 01KZ9YAP5KRPXSW7717WZ3JEA8)
- 2026-08-05T23:27:49Z accepted by a-root
- 2026-08-05T23:27:49Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-05T23:27:49Z completed by a-root
- 2026-08-06T00:14:52Z status done proposed by a-fixer-nxt1h2, applied (event 01KZA32AS91A4Q7FA59NSP7KJW)
- 2026-08-06T00:14:52Z status done proposed by a-fixer-evg4ws, applied (event 01KZA38R58CYNAG7FA6CE3T2Z0)
- 2026-08-06T00:14:52Z status done proposed by a-fixer-9efq2w, applied (event 01KZA3D5TDX4BX6GVDT9YCG9K0)
- 2026-08-06T00:14:52Z status done proposed by a-fixer-mk6ege, applied (event 01KZA3FXA94A9GJT6AWA6M8D52)
- 2026-08-06T00:14:52Z status done proposed by a-fixer-vx7753, applied (event 01KZA3JYVQ67RAGAAKB3E6V7K6)
- 2026-08-06T00:14:52Z status done proposed by a-fixer-xxkhh3, applied (event 01KZA3MP9BDW5RZ74H31T1PMGK)
- 2026-08-06T00:14:52Z status done proposed by a-fixer-dgadk8, applied (event 01KZA3RKNSFZZQHZH77YHMJJXJ)
- 2026-08-06T00:14:52Z status done proposed by a-fixer-2wavrv, applied (event 01KZA3TBDQJ93TTME6NA5HBAMT)
- 2026-08-06T00:14:55Z accepted by a-root
- 2026-08-06T00:14:55Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-06T00:14:55Z completed by a-root
- 2026-08-10T16:54:59Z accepted by a-root
- 2026-08-10T16:54:59Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T16:54:59Z deliverable: dacli/007-describe-schema-commands-emit-the-tool-catalog-with-json-schemas is merged into trunk
- 2026-08-10T16:54:59Z completed by a-root
