---
id: d-schema-cmd-emits-a-draft-2020-12-json-schema-doc-nesting-the-tool-s-input
kind: note
note_kind: decision
created: 2026-08-05T21:39:22Z
created_by: a-fixer-x4yg3c
about: "[[007]]"
---
# schema <cmd> emits a draft 2020-12 JSON Schema doc nesting the tool's input+output; describe --json emits the tools array
## Chose
schema <cmd> emits a draft 2020-12 JSON Schema doc nesting the tool's input+output; describe --json emits the tools array
## Rejected
Returning just the input schema, or the raw Tool struct, from schema <cmd>
## Because
A command's typed IO is both what the caller supplies (input) and what it returns (output); nesting both under one titled draft-2020-12 doc is self-describing and lets an agent validate either side. describe --json returns the bare tools array (name/summary/inputSchema/outputSchema) so an agent maps it straight onto its tool list, while bare 'describe' still returns the full {version,tools} catalog.
