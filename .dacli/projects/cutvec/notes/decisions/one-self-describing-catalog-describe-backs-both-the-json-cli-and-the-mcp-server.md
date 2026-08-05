---
id: d-one-self-describing-catalog-describe-backs-both-the-json-cli-and-the-mcp-server
kind: note
note_kind: decision
created: 2026-08-05T13:27:09Z
created_by: a-root
---
# One self-describing catalog (describe) backs BOTH the JSON CLI and the MCP server
## Chose
One self-describing catalog (describe) backs BOTH the JSON CLI and the MCP server
## Rejected
CLI-only; MCP-only
## Because
describe emits the tool catalog + JSON Schemas that generate both surfaces so they never drift.
