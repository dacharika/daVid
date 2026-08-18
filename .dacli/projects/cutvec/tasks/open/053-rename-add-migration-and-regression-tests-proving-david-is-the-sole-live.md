---
id: t-01M0ADTVCPHB01Y1CS2EM9N05S
kind: task
created: 2026-08-18T12:34:10Z
created_by: a-root
owner: a-root
priority: must
estimate: "{optimistic: 3, probable: 5, pessimistic: 8}"
depends_on: "[052]"
---
# rename: add migration and regression tests proving daVid is the sole live identity and cutvec compatibility paths work
## So that
a case-insensitive search for cutvec in the live codebase returns only intentional compatibility sites, each documented, and the test suite proves the migration did not break existing projects or worker contracts
## Acceptance
- [ ] go test ./... passes a test that builds cmd/daVid and asserts the binary help output contains daVid and no bare cutvec references
- [ ] a test proves DAVID_* takes precedence over CUTVEC_* when both are set and that CUTVEC_* alone still works
- [ ] grep -ri cutvec . --include='*.go' --include='*.py' --include='*.md' --include='*.yml' outside .dacli/ and intentional compatibility sites returns no hits
- [ ] go test ./... -count=1 and go test -race ./... both exit 0
- [ ] ruff check workers/ and ruff format --check workers/ exit 0
## Log
