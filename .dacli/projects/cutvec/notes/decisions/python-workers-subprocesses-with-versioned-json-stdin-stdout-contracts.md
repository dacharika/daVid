---
id: d-python-workers-subprocesses-with-versioned-json-stdin-stdout-contracts
kind: note
note_kind: decision
created: 2026-08-05T13:27:09Z
created_by: a-root
---
# Python workers = subprocesses with versioned JSON stdin/stdout contracts
## Chose
Python workers = subprocesses with versioned JSON stdin/stdout contracts
## Rejected
cgo MLT bindings; a gRPC service
## Because
subprocess+JSON is simplest, most deterministic, cacheable and testable; cgo is fragile, gRPC overkill for v1.
