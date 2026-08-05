---
id: d-mlt-xml-timeline-is-the-agent-editable-artifact-rendered-via-melt
kind: note
note_kind: decision
created: 2026-08-05T13:27:09Z
created_by: a-root
---
# MLT XML timeline is the agent-editable artifact, rendered via melt
## Chose
MLT XML timeline is the agent-editable artifact, rendered via melt
## Rejected
imperative ffmpeg command-string generation
## Because
declarative, inspectable, diffable, deterministic object model an agent can mutate and validate; far better AX than opaque ffmpeg strings.
