---
id: d-filed-the-mcp-assets-tool-routing-gap-assets-uninvokable-over-mcp-as-the-single
kind: note
note_kind: decision
created: 2026-08-10T21:24:36Z
created_by: a-go-auditor-hh08zq
about: "[[006]]"
---
# Filed the MCP assets-tool routing gap (assets uninvokable over MCP) as the single highest-value evidence-based change
## Chose
Filed the MCP assets-tool routing gap (assets uninvokable over MCP) as the single highest-value evidence-based change
## Rejected
Re-filing any open task 031-036 (multi-clip sequencing, blank element, render atomic publish, shorts rank, assets-search Go-side rank, Clock :60.000 rounding) — all near-dups dacli refuses (exit 3); or the applyAudio map-iteration tie-break nit (rejected repeatedly: rare, needs an exact float tie, hard to pin deterministically)
## Because
Every determinism/audio/render finding in the brief is now an open task (031-036), so re-filing any is a near-dup. The MCP assets-routing gap is DISTINCT from all of them and from done 015 (assets CLI) / 021 (MCP describe/timeline/schema), and PROVEN from source with no melt/lancedb: tools/list advertises 'assets' (mcp.go:174-182 over catalog.go:341-366) but callTool (mcp.go:211-256) appends a subcommand only for 'schema' and 'timeline', never 'assets', so run() calls assetsCmd([]) (main.go:87-88) which returns input.empty 'no assets subcommand given' (assets.go:69-73) before any worker — making BOTH assets index and assets search unreachable over MCP. It hits the headline 'MCP server exposes the catalog as MCP tools' promise on the entire library/SFX retrieval surface the CLI supports but MCP silently cannot reach. Unit-testable in pure Go via the existing serveMCP helper with no worker; mcp_test.go asserts nothing about assets today, so a fix ships green. Fix mirrors timeline 'op' routing: add an assets branch in callTool reading an op enum [index,search] and add that op to the assets InputSchema.
