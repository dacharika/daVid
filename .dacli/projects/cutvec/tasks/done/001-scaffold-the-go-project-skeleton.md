---
id: t-01KZ91JE4HHBWVT92F6KFT22V3
kind: task
created: 2026-08-05T13:24:56Z
created_by: a-root
owner: a-root
priority: must
estimate: "{optimistic: 1, probable: 2, pessimistic: 4}"
---
# Scaffold the Go project skeleton
## So that
every later task starts from a repository that already builds
## Context
Run `go mod init <module-path>`. Target layout: cmd/ for entry points, internal/ for packages the module owns, one package per bounded concern.
## Acceptance
- [x] The starting layout exists: cmd/ for entry points, internal/ for packages the module owns, one package per bounded concern
- [x] `go build ./...` exits 0 from a clean checkout
- [x] A .gitignore excludes build output and local environment files
## Log
- 2026-08-05T14:23:52Z claimed by a-7p3edw
- 2026-08-05T14:29:10Z accepted by a-root
- 2026-08-05T14:29:10Z verified by `go build ./...` (exit 0)
- 2026-08-05T14:29:10Z completed by a-root
