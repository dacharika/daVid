---
id: t-01KZ9WMYPK5WD7QHZSJ3HA5WR7
kind: task
created: 2026-08-05T21:18:10Z
created_by: a-root
owner: a-root
priority: must
estimate: "{optimistic: 3, probable: 5, pessimistic: 8}"
---
# python worker protocol: versioned JSON stdin/stdout + Go invoker
## So that
Go orchestrates deterministic Python ML workers over a stable contract
## Acceptance
- [ ] internal/worker invokes a python worker passing a JSON request on stdin and parsing a JSON response on stdout
- [ ] protocol carries a version field and errors map to axerror {code,hint,retryable}
- [ ] a fake echo worker round-trips in a go test
## Log
- 2026-08-05T21:37:09Z claimed by a-fixer-4qganh
