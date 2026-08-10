---
id: t-01KZ9WMYY518SGQK9WVYTRQW0D
kind: task
created: 2026-08-05T21:18:10Z
created_by: a-root
owner: a-root
priority: should
estimate: "{optimistic: 8, probable: 13, pessimistic: 20}"
---
# shorts recipe: vertical shorts with hook + karaoke captions
## So that
one call reproduces speaker-locked captioned shorts from a search query
## Acceptance
- [x] cutvec shorts <project> --query '<q>' --json produces 9:16 clips with burned-in karaoke captions and a hook
## Log
- 2026-08-10T15:05:23Z accepted by a-root
- 2026-08-10T15:05:23Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T15:05:23Z deliverable: no dacli/019-shorts-recipe-vertical-shorts-with-hook-karaoke-captions branch — nothing to check against trunk
- 2026-08-10T15:05:23Z completed by a-root
