---
id: t-01M0ADTHHVFSQAYSJ139HXGJDM
kind: task
created: 2026-08-18T12:34:00Z
created_by: a-root
owner: a-root
priority: must
estimate: "{optimistic: 2, probable: 3, pessimistic: 4}"
depends_on: "[051]"
---
# rename: update README, AGENTS.md, CONTRIBUTING.md, workers/README.md and smoke_test.sh to daVid identity
## So that
a reader who clones the repo sees daVid throughout the documentation and can follow the README install and quickstart commands without hitting stale cutvec names
## Acceptance
- [ ] README.md title, install command, quickstart examples, and env var table all read daVid
- [ ] AGENTS.md, CONTRIBUTING.md, and workers/README.md contain no live cutvec references outside historical notes
- [ ] workers/smoke_test.sh CLI variable points at cmd/daVid and the banner prints daVid
- [ ] grep -rn cutvec README.md AGENTS.md CONTRIBUTING.md workers/README.md workers/smoke_test.sh returns no hits outside intentional deprecation notes
- [ ] go build ./... and go test ./... exit 0
## Log
