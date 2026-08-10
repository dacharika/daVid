---
id: t-01KZPDSSRZMHR2CJNA1DSP6CNV
kind: task
created: 2026-08-10T18:08:47Z
created_by: a-go-auditor-zd1s8t
owner: a-go-auditor-zd1s8t
priority: should
---
# shorts: stop the hook cue from overlapping the first caption cue in buildSRT
## So that
the hook card stands alone in the opening 2.5s instead of rendering stacked with the first karaoke captions
## Acceptance
- [ ] buildSRT produces no two cues whose [start,end) spans overlap, hook present or absent — proven by a new Go unit test that parses the SRT output and compares consecutive cue spans, covering the case where the first spoken word begins before the 2.5s hook ends
- [ ] The hook, when present, is still the opening cue and readable (TestBuildSRTLeadsWithTheHook still passes or is updated to also assert non-overlap)
- [ ] go build ./... and go test -race ./... both exit 0; gofmt, go vet, and staticcheck clean
## Log
