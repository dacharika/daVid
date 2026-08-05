---
id: d-ax-contract-handles-not-blobs-deterministic-idempotent-resumable-content-hash
kind: note
note_kind: decision
created: 2026-08-05T13:27:31Z
created_by: a-root
---
# AX contract: handles-not-blobs, deterministic/idempotent/resumable (content-hash cache), structured errors {code,hint,retryable}, --plan dry-run, JSONL progress, explicit project-dir state
## Chose
AX contract: handles-not-blobs, deterministic/idempotent/resumable (content-hash cache), structured errors {code,hint,retryable}, --plan dry-run, JSONL progress, explicit project-dir state
## Rejected
human-facing UX (pretty tables, spinners, prose output)
## Because
primary user is an AI agent; token-frugal, recoverable, replayable IO is the product, not a UI.
