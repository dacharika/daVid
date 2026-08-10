---
id: t-01KZ9WMYZNTJV4QS45V7KR7H2N
kind: task
created: 2026-08-05T21:18:10Z
created_by: a-root
owner: a-root
priority: should
estimate: "{optimistic: 3, probable: 5, pessimistic: 8}"
---
# mcp serve exposes the catalog as MCP tools over stdio
## So that
agents call cutvec as native MCP tools with the same schemas as the CLI
## Acceptance
- [x] cutvec mcp serve exposes each describe tool as an MCP tool; tools/list matches the catalog
## Log
- 2026-08-10T15:05:24Z accepted by a-root
- 2026-08-10T15:05:24Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T15:05:24Z deliverable: no dacli/021-mcp-serve-exposes-the-catalog-as-mcp-tools-over-stdio branch — nothing to check against trunk
- 2026-08-10T15:05:24Z completed by a-root
