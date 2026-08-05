---
id: d-module-path-github-com-tahabsn-cutvec-single-binary-cmd-cutvec-exposes-both-cli
kind: note
note_kind: decision
created: 2026-08-05T14:24:53Z
created_by: a-7p3edw
about: "[[001]]"
---
# Module path github.com/tahabsn/cutvec; single binary cmd/cutvec exposes both CLI and MCP surfaces
## Chose
Module path github.com/tahabsn/cutvec; single binary cmd/cutvec exposes both CLI and MCP surfaces
## Rejected
Separate cmd/cutvec-cli and cmd/cutvec-mcp binaries
## Because
One self-describing catalog backs both surfaces (d-one-self-describing-catalog...); a single binary keeps distribution to one artifact and prevents the two surfaces from drifting.
