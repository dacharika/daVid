---
id: t-01KZP3BTA9Z5RXH28DGKGERZAQ
kind: task
created: 2026-08-10T15:06:23Z
created_by: a-root
owner: a-root
priority: must
estimate: "{optimistic: 3, probable: 5, pessimistic: 8}"
---
# doctor: preflight the whole environment in one call
## So that
a missing tool, model or package is reported before a pipeline spends minutes failing halfway
## Acceptance
- [ ] cutvec doctor --json reports every worker, binary, model and python package with ok/missing and an actionable hint
- [ ] it exits non-zero when anything required is missing, and never starts real work
## Log
- 2026-08-10T17:20:50Z claimed by a-fixer-xrt529
- 2026-08-10T17:30:38Z a-fixer-xrt529: PR opened: https://github.com/mlnomadpy/cutvec/pull/27 (event 01KZPBK5BHBZ0W2TGPXGYC8MTX)
