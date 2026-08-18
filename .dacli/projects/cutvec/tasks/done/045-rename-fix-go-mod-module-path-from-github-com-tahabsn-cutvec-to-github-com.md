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
- [x] go.mod declares module github.com/mlnomadpy/daVid
- [x] every .go file imports from github.com/mlnomadpy/daVid (grep for tahabsn/cutvec returns no hits)
- [x] go mod tidy exits 0 with no unrelated dependency churn
- [x] go build ./... and go test ./... both exit 0 after the rename
- [x] CI assertion: rg 'github.com/tahabsn/cutvec|github.com/mlnomadpy/cutvec' --glob '*.go' go.mod returns no matches
## Log
- 2026-08-17T03:17:19Z claimed by a-eyp93hppat
- 2026-08-17T12:18:36Z a-vcd9cyxvam: PR opened: https://github.com/mlnomadpy/daVid/pull/67 (event 01M07T3JJ37W3AJAGDXNNKS3Y8)
- 2026-08-17T12:18:36Z status done proposed by a-vcd9cyxvam, applied (event 01M07T3STQQSHWHH4QGKG5EY34)
- 2026-08-17T12:18:49Z accepted by a-root
- 2026-08-17T12:18:49Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-17T12:18:49Z completed by a-root
