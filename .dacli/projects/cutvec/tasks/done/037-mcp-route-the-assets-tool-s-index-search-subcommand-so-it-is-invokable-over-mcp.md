---
id: t-01KZPS2PS48NEFW4VMHJC24ZDN
kind: task
created: 2026-08-10T21:25:54Z
created_by: a-go-auditor-hh08zq
owner: a-root
priority: should
estimate: "{optimistic: 1, probable: 2, pessimistic: 4}"
---
# mcp: route the assets tool's index/search subcommand so it is invokable over MCP
## So that
an agent can actually run assets index and assets search through the MCP server, not just the CLI — today the assets tool is advertised in tools/list but every tools/call for it returns input.empty 'no assets subcommand given' before any worker, so the whole library/SFX/b-roll retrieval surface is silently unreachable over MCP, contradicting the headline 'MCP server exposes the catalog as MCP tools' promise. Root cause: callTool (cmd/cutvec/mcp.go:211-256) appends a positional subcommand only for 'schema' (mcp.go:219) and 'timeline' (mcp.go:232 via an 'op' field), never for 'assets', so run() calls assetsCmd([]) (cmd/cutvec/main.go:87-88) which errors at assets.go:69-73; and the catalog assets InputSchema (internal/catalog/catalog.go:344-355) has no field naming the subcommand. Fix mirrors the timeline 'op' routing. Distinct from open 029-036 and done 015/021. See finding [[f-assets-tool-is-uninvokable-over-mcp-calltool-never-routes-its-index-search]].
## Acceptance
- [x] callTool (cmd/cutvec/mcp.go) reads an 'op' selector (index|search) from the assets arguments and appends it as the positional subcommand, mirroring the existing timeline 'op' routing, so an MCP tools/call name=assets reaches assetsIndexCmd/assetsSearchCmd
- [x] The catalog assets InputSchema (internal/catalog/catalog.go) documents an 'op' field with enum [index,search] so an MCP client can select which subcommand runs
- [x] A new pure-Go test in cmd/cutvec/mcp_test.go drives serveMCP with tools/call name=assets op=search (and op=index) and asserts the result does NOT contain 'no assets subcommand given' — the test fails on current main and passes after the fix; no melt/lancedb needed
- [x] go build ./... and go test ./... both exit 0; gofmt clean on every touched file
## Log
- 2026-08-10T21:27:50Z adopted by a-root (owner a-go-auditor-hh08zq orphaned)
- 2026-08-11T09:52:30Z accepted by a-root
- 2026-08-11T09:52:30Z verified by `go build ./... && go test ./... -count=1` (exit 0)
- 2026-08-11T09:52:30Z deliverable: no dacli/037-mcp-route-the-assets-tool-s-index-search-subcommand-so-it-is-invokable-over-mcp branch — nothing to check against main
- 2026-08-11T09:52:30Z completed by a-root
