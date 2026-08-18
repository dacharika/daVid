---
id: t-01M0ADT848AJZQ55BZKJ24Y70H
kind: task
created: 2026-08-18T12:33:51Z
created_by: a-root
owner: a-root
priority: must
estimate: "{optimistic: 1, probable: 2, pessimistic: 3}"
depends_on: "[050]"
---
# rename: replace cutvec brand references in catalog, axerror, project, timeline, and worker package docs
## So that
every agent-visible hint, package comment, and error recovery line identifies the product as daVid so agents can act on them without encountering stale branding
## Acceptance
- [ ] grep -rn cutvec internal/ --include='*.go' returns only intentional compatibility comments (none in package docs, summaries, or actionable hints)
- [ ] every recovery hint that says 'run cutvec X' now says 'run daVid X' and is executable as written
- [ ] daVid describe and daVid schema output contain no cutvec references in agent-visible fields
- [ ] go build ./... and go test ./... both exit 0
## Log
