---
id: t-01M06VE84DYFZN3C18SNEBMAQ1
kind: task
created: 2026-08-17T03:15:00Z
created_by: a-root
owner: a-root
priority: must
estimate: "{optimistic: 2, probable: 3, pessimistic: 5}"
depends_on: "[045]"
---
# rename: move cmd/cutvec/ to cmd/daVid/ and update help banners, usage text, examples, and build target
## So that
the canonical CLI entry point, built binary, and every user-visible hint identify the product as daVid instead of cutvec
## Acceptance
- [ ] cmd/cutvec/ directory is gone; cmd/daVid/ contains the same files with updated package comments
- [ ] every help banner, usage line, and actionable hint in cmd/daVid/ reads daVid (grep for 'cutvec' in cmd/daVid/ returns no hits outside intentional compatibility notes)
- [ ] go build -o build/daVid ./cmd/daVid exits 0 and the binary self-identifies as daVid in its usage output
- [ ] mcp.go serverInfo.name is 'daVid' and mcp_test.go asserts this
- [ ] go build ./... and go test ./... -count=1 both exit 0
## Log
