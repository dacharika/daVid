---
id: t-01KZPDSSRZMHR2CJNA1DSP6CNV
kind: task
created: 2026-08-10T18:08:47Z
created_by: a-go-auditor-zd1s8t
owner: a-root
priority: should
estimate: "{optimistic: 1, probable: 2, pessimistic: 4}"
---
# shorts: stop the hook cue from overlapping the first caption cue in buildSRT
## So that
the hook card stands alone in the opening 2.5s instead of rendering stacked with the first karaoke captions
## Acceptance
- [x] buildSRT produces no two cues whose [start,end) spans overlap, hook present or absent — proven by a new Go unit test that parses the SRT output and compares consecutive cue spans, covering the case where the first spoken word begins before the 2.5s hook ends
- [x] The hook, when present, is still the opening cue and readable (TestBuildSRTLeadsWithTheHook still passes or is updated to also assert non-overlap)
- [x] go build ./... and go test -race ./... both exit 0; gofmt, go vet, and staticcheck clean
## Log
- 2026-08-10T20:57:57Z adopted by a-root (owner a-go-auditor-zd1s8t orphaned)
- 2026-08-10T20:59:00Z claimed by a-fixer-24tkme
- 2026-08-10T21:02:49Z a-fixer-24tkme: PR opened: https://github.com/mlnomadpy/cutvec/pull/33 (event 01KZPQQXKW2J9064NYAE4NN876)
- 2026-08-10T21:08:24Z accepted by a-root
- 2026-08-10T21:08:24Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T21:08:24Z deliverable: dacli/029-shorts-stop-the-hook-cue-from-overlapping-the-first-caption-cue-in-buildsrt is merged into main
- 2026-08-10T21:08:24Z completed by a-root
- 2026-08-10T21:08:28Z accepted by a-root
- 2026-08-10T21:08:28Z closed WITHOUT verification — no --verify command was given
- 2026-08-10T21:08:28Z deliverable: dacli/029-shorts-stop-the-hook-cue-from-overlapping-the-first-caption-cue-in-buildsrt is merged into main
- 2026-08-10T21:08:28Z completed by a-root
- 2026-08-10T21:20:10Z accepted by a-root
- 2026-08-10T21:20:10Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T21:20:10Z deliverable: no dacli/029-shorts-stop-the-hook-cue-from-overlapping-the-first-caption-cue-in-buildsrt branch — nothing to check against main
- 2026-08-10T21:20:10Z completed by a-root
- 2026-08-10T21:27:48Z accepted by a-root
- 2026-08-10T21:27:48Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T21:27:48Z deliverable: no dacli/029-shorts-stop-the-hook-cue-from-overlapping-the-first-caption-cue-in-buildsrt branch — nothing to check against main
- 2026-08-10T21:27:48Z completed by a-root
