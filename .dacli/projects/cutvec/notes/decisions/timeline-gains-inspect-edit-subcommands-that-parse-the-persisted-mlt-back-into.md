---
id: d-timeline-gains-inspect-edit-subcommands-that-parse-the-persisted-mlt-back-into
kind: note
note_kind: decision
created: 2026-08-10T18:05:13Z
created_by: a-fixer-0mj66x
about: "[[026]]"
---
# timeline gains inspect/edit subcommands that parse the persisted .mlt back into the model and mutate it, reusing the build path's effect resolution
## Chose
timeline gains inspect/edit subcommands that parse the persisted .mlt back into the model and mutate it, reusing the build path's effect resolution
## Rejected
Adding separate top-level tools (timeline-inspect/timeline-edit), or re-deriving the timeline from search+detection to change one filter
## Because
The .mlt already round-trips through encoding/xml, so timeline.Parse is Marshal's inverse; inspect/edit as positional subcommands mirror the existing assets index/search precedent and keep one catalog tool. edit reuses resolveEffects/applyEffects (same producer-existence checks as build) and re-validates before writing, so a mutation that would break render fails without touching the file. MCP reaches the subcommands via an 'op' field in callTool, mirroring how schema is special-cased.
