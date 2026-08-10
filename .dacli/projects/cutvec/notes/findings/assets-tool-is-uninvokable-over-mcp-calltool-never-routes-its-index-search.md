---
id: f-assets-tool-is-uninvokable-over-mcp-calltool-never-routes-its-index-search
kind: note
note_kind: finding
created: 2026-08-10T21:18:29Z
created_by: a-go-auditor-cakjp6
about: "[[006]]"
severity: major
---
# assets tool is uninvokable over MCP: callTool never routes its index/search subcommand
The MCP server lists 'assets' in tools/list (dispatchMCP tools/list at cmd/cutvec/mcp.go:174-182 mirrors the whole catalog, which includes the assets tool at internal/catalog/catalog.go:341-366), but a tools/call for it can never succeed. callTool (cmd/cutvec/mcp.go:211-256) builds args := []string{name} = ["assets"] and only appends a positional subcommand for two tools: 'schema' (mcp.go:219) and 'timeline' (mcp.go:232, reading an "op" of inspect/edit). There is NO such routing for 'assets'. run() then dispatches case "assets": assetsCmd(args[1:]) (cmd/cutvec/main.go:87-88) with args[1:] == [], and assetsCmd (cmd/cutvec/assets.go:69-73) returns axerror input.empty 'no assets subcommand given' the moment len(args)==0 — before any worker resolves. So BOTH 'assets index' and 'assets search' are unreachable over MCP; every assets tools/call returns the isError 'no assets subcommand given'. Root cause is twofold: (1) callTool has no assets branch, and (2) the catalog assets InputSchema (catalog.go:344-355) has no field naming the subcommand — dir/query are documented as positional ('also accepted as the positional argument'), which MCP arguments cannot supply, so even a client that read the schema has no way to select index vs search. Proven from source, no melt/lancedb needed: it fails at arg routing. LATENT/GREEN: mcp_test.go has zero 'assets' assertions (TestMCPToolsCallRunsTheTool only drives 'describe'); a serveMCP(t, tools/call name=assets, arguments={project,dir}) test asserting it does NOT return 'no assets subcommand given' fails today and passes after the fix. DISTINCT from open 030-036 (audio/render/rank/clock determinism) and done 015 (assets CLI happy path) / 021 (MCP serve happy path — describe/timeline/schema only).
