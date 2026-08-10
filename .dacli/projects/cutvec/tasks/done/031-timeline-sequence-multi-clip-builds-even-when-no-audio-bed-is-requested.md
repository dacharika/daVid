---
id: t-01KZPFZW2RA5ACV7SZHC847RHD
kind: task
created: 2026-08-10T18:47:04Z
created_by: a-go-auditor-emr68x
owner: a-root
priority: should
estimate: "{optimistic: 1, probable: 2, pessimistic: 4}"
---
# timeline: sequence multi-clip builds even when no audio bed is requested
## So that
a multi-clip 'timeline' build plays as one ordered edit instead of silently losing every clip but one whenever no music bed is requested
## Acceptance
- [x] A multi-clip (2+) 'timeline' build with NO audio yields tl.Tractor != nil and tl.Playlists containing a playlist with one entry per clip, ordered as requested (evidence: today timelineBuildCmd main.go:229-256 never calls tl.Sequence(); sequencing only happens inside AddAudio timeline.go:92-96, so a no-audio build writes bare <producer>s with no <playlist>/<tractor>)
- [x] A new Go test builds the multi-clip no-audio case and asserts the playlist+tractor structure with clip order preserved; it fails against current main and passes after the fix (TestTimelineSlice main_test.go:301-342 currently checks only resources/ids and lets the defect ship green)
- [x] The audio path is unchanged in effect: a build WITH audio still yields the mix tractor (video track + bed track + mix transition) and still validates; AddAudio's nil-tractor guard is kept as a safety net
- [x] go build ./... and go test ./... both exit 0; gofmt clean; distinct from tasks 029 (hook/caption SRT overlap) and 030 (audio bed gain) — do not touch their scope
## Log
- 2026-08-10T20:57:57Z adopted by a-root (owner a-go-auditor-emr68x orphaned)
- 2026-08-10T21:20:14Z claimed by a-fixer-tg912m
- 2026-08-10T21:23:15Z a-fixer-tg912m: PR opened: https://github.com/mlnomadpy/cutvec/pull/35 (event 01KZPRXBVPPHQCJ5BA3D9CZ9DG)
- 2026-08-10T21:27:49Z accepted by a-root
- 2026-08-10T21:27:49Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T21:27:49Z deliverable: dacli/031-timeline-sequence-multi-clip-builds-even-when-no-audio-bed-is-requested is merged into main
- 2026-08-10T21:27:49Z completed by a-root
- 2026-08-10T21:27:52Z accepted by a-root
- 2026-08-10T21:27:52Z closed WITHOUT verification — no --verify command was given
- 2026-08-10T21:27:52Z deliverable: dacli/031-timeline-sequence-multi-clip-builds-even-when-no-audio-bed-is-requested is merged into main
- 2026-08-10T21:27:52Z completed by a-root
- 2026-08-10T21:37:09Z accepted by a-root
- 2026-08-10T21:37:09Z verified by `go build ./... && go test ./...` (exit 0)
- 2026-08-10T21:37:09Z deliverable: no dacli/031-timeline-sequence-multi-clip-builds-even-when-no-audio-bed-is-requested branch — nothing to check against main
- 2026-08-10T21:37:09Z completed by a-root
