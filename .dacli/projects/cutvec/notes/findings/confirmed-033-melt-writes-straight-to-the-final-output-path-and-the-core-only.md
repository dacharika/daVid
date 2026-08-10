---
id: f-confirmed-033-melt-writes-straight-to-the-final-output-path-and-the-core-only
kind: note
note_kind: finding
created: 2026-08-10T19:13:14Z
created_by: a-root
origin: workers/melt_worker.py:handle_render
---
# CONFIRMED 033: melt writes straight to the final output path and the core only checks existence, so a crashed render leaves a truncated file that is cached as valid
