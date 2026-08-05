---
id: d-bounded-concerns-as-internal-packages-catalog-axerror-project-worker-timeline
kind: note
note_kind: decision
created: 2026-08-05T14:24:57Z
created_by: a-7p3edw
about: "[[001]]"
---
# Bounded concerns as internal/ packages: catalog, axerror, project, worker, timeline
## Chose
Bounded concerns as internal/ packages: catalog, axerror, project, worker, timeline
## Rejected
A single flat internal package, or fully empty placeholder dirs
## Because
The spec names distinct bounded concerns (self-describing catalog, structured errors, project-dir state, Python worker JSON contract, MLT XML timeline). One package per concern with a minimal real type establishes import boundaries with no cycles, so later tasks flesh out a package without moving files.
