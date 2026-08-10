---
id: t-01KZPFZW2RA5ACV7SZHC847RHD
kind: task
created: 2026-08-10T18:47:04Z
created_by: a-go-auditor-emr68x
owner: a-go-auditor-emr68x
priority: should
---
# timeline: sequence multi-clip builds even when no audio bed is requested
## So that
a multi-clip 'timeline' build plays as one ordered edit instead of silently losing every clip but one whenever no music bed is requested
## Acceptance
- [ ] A multi-clip (2+) 'timeline' build with NO audio yields tl.Tractor != nil and tl.Playlists containing a playlist with one entry per clip, ordered as requested (evidence: today timelineBuildCmd main.go:229-256 never calls tl.Sequence(); sequencing only happens inside AddAudio timeline.go:92-96, so a no-audio build writes bare <producer>s with no <playlist>/<tractor>)
- [ ] A new Go test builds the multi-clip no-audio case and asserts the playlist+tractor structure with clip order preserved; it fails against current main and passes after the fix (TestTimelineSlice main_test.go:301-342 currently checks only resources/ids and lets the defect ship green)
- [ ] The audio path is unchanged in effect: a build WITH audio still yields the mix tractor (video track + bed track + mix transition) and still validates; AddAudio's nil-tractor guard is kept as a safety net
- [ ] go build ./... and go test ./... both exit 0; gofmt clean; distinct from tasks 029 (hook/caption SRT overlap) and 030 (audio bed gain) — do not touch their scope
## Log
