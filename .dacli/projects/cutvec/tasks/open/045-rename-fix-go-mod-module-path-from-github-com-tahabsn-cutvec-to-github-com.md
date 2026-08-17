---
id: t-01M06VDZVMMTF51BJT14XMZC1N
kind: task
created: 2026-08-17T03:14:51Z
created_by: a-root
owner: a-root
priority: must
estimate: "{optimistic: 1, probable: 2, pessimistic: 3}"
---
# rename: fix go.mod module path from github.com/tahabsn/cutvec to github.com/mlnomadpy/daVid and update all internal imports
## So that
the repository's Go module identity matches the canonical product name after the GitHub rename, and every internal import resolves without an alias
## Acceptance
- [ ] go.mod declares module github.com/mlnomadpy/daVid
- [ ] every .go file imports from github.com/mlnomadpy/daVid (grep for tahabsn/cutvec returns no hits)
- [ ] go mod tidy exits 0 with no unrelated dependency churn
- [ ] go build ./... and go test ./... both exit 0 after the rename
- [ ] CI assertion: rg 'github.com/tahabsn/cutvec|github.com/mlnomadpy/cutvec' --glob '*.go' go.mod returns no matches
## Log
- 2026-08-17T03:17:19Z claimed by a-eyp93hppat
