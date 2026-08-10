---
id: d-filed-the-mcp-assets-subcommand-routing-gap-assets-tool-uninvokable-over-mcp-as
kind: note
note_kind: decision
created: 2026-08-10T21:18:41Z
created_by: a-go-auditor-cakjp6
about: "[[006]]"
---
# Filed the MCP assets-subcommand routing gap (assets tool uninvokable over MCP) as the single highest-value evidence-based change
## Chose
Filed the MCP assets-subcommand routing gap (assets tool uninvokable over MCP) as the single highest-value evidence-based change
## Rejected
Re-filing any open task 030-036 (audio bed gain, multi-clip sequencing, blank element, render atomic publish, shorts rank, assets-search Go-side rank, Clock :60.000 rounding) — all near-dups dacli refuses; or the repeatedly-rejected applyAudio/speakerAt map-iteration tie-break nit
## Because
Every determinism/audio/render finding surfaced in the brief is now an open task (030-036), so re-filing any is a near-duplicate (exit 3). The MCP assets-routing gap is DISTINCT from all of them and from done 015 (assets CLI) / 021 (MCP serve happy path), and is PROVEN from source with no melt/lancedb: tools/list advertises 'assets' (mcp.go:174-182 over catalog.go:341-366) but callTool (mcp.go:211-256) only appends a positional subcommand for 'schema' and 'timeline', never 'assets', so run() calls assetsCmd([]) (main.go:87-88) which returns input.empty 'no assets subcommand given' (assets.go:69-73) before any worker — making BOTH assets index and assets search unreachable over MCP. It hits the headline 'MCP server exposes the catalog as MCP tools' promise on the entire library/SFX retrieval surface, which the CLI supports but MCP silently cannot reach. It is unit-testable in pure Go via the existing serveMCP helper (mcp_test.go) with no worker, and mcp_test.go asserts nothing about assets today, so a fix ships green. The fix mirrors the existing timeline 'op' routing: add an assets branch in callTool that reads a subcommand selector and appends index/search, and add that selector (an op enum [index,search]) to the assets InputSchema so an agent can choose it.
