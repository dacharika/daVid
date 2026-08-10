---
id: f-assets-tool-is-uninvokable-over-mcp-calltool-never-routes-its-index-search-149r1w
kind: note
note_kind: finding
created: 2026-08-10T21:24:27Z
created_by: a-go-auditor-hh08zq
about: "[[006]]"
severity: major
---
# assets tool is uninvokable over MCP: callTool never routes its index/search subcommand
tools/list advertises 'assets' (cmd/cutvec/mcp.go:174-182 mirrors catalog.Default, which includes the assets tool at internal/catalog/catalog.go:341-366), but every tools/call for it fails. callTool (cmd/cutvec/mcp.go:211-256) builds args:=[]string{name}=["assets"] and appends a positional subcommand ONLY for 'schema' (mcp.go:219) and 'timeline' (mcp.go:232, reading an 'op'). There is NO assets branch. run() then dispatches case "assets": assetsCmd(args[1:]) (cmd/cutvec/main.go:87-88) with args[1:]==[], and assetsCmd (cmd/cutvec/assets.go:69-73) returns axerror input.empty 'no assets subcommand given' the moment len(args)==0 — before any worker. So BOTH 'assets index' and 'assets search' are unreachable over MCP. Twofold root cause: (1) callTool has no assets branch; (2) the catalog assets InputSchema (catalog.go:344-355) has no field naming the subcommand — dir/query are 'index only'/'search only' positionals MCP arguments cannot supply, so a schema-reading client cannot select index vs search either. Proven from source, no melt/lancedb — fails at arg routing. LATENT/GREEN: mcp_test.go has zero assets assertions (TestMCPToolsCallRunsTheTool drives only 'describe'); a serveMCP tools/call name=assets test asserting it does NOT return 'no assets subcommand given' fails today, passes after the fix. Fix mirrors timeline 'op' routing: add an assets branch in callTool that reads an op selector (index|search) and appends it, and add that op enum to the assets InputSchema. DISTINCT from open 029-036 (audio/render/rank/clock/sequencing/blank determinism) and done 015 (assets CLI happy path) / 021 (MCP serve happy path: describe/timeline/schema only).
