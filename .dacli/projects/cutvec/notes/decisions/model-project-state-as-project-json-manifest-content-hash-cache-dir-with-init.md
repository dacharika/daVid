---
id: d-model-project-state-as-project-json-manifest-content-hash-cache-dir-with-init
kind: note
note_kind: decision
created: 2026-08-05T23:08:05Z
created_by: a-fixer-kthys7
about: "[[008]]"
---
# Model project state as project.json manifest + content-hash cache/ dir, with init/ingest/status as three CLI tools
## Chose
Model project state as project.json manifest + content-hash cache/ dir, with init/ingest/status as three CLI tools
## Rejected
A single embedded state blob returned by status, or per-step ad-hoc files with no shared manifest
## Because
The manifest is the durable, diffable record every step reads/writes; ingest keys sources on sha256(bytes) so re-ingest is a cache hit (idempotent/resumable per the AX contract), and status resolves artifacts done/pending from disk. One manifest keeps the graph readable in a single JSON call, matching internal/project's existing CacheDir()/handles-not-blobs design.
