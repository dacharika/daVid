---
id: f-multi-clip-no-audio-timeline-builds-now-sequence-into-a-playlist-tractor
kind: note
note_kind: finding
created: 2026-08-10T21:22:49Z
created_by: a-fixer-tg912m
about: "[[031]]"
severity: major
---
# Multi-clip no-audio timeline builds now sequence into a playlist+tractor
Root cause: timelineBuildCmd (cmd/cutvec/main.go:229-256) appended producers but never called tl.Sequence(); sequencing only happened as a side effect of AddAudio's nil-tractor guard (internal/timeline/timeline.go:97-101). So a build with NO audio marshaled bare <producer>s with no <playlist>/<tractor> and melt plays only the first clip. Fix: after applyEffects and before applyAudio, 'if len(tl.Producers) >= 2 { tl.Sequence("main") }' (main.go). Sequence('main') builds a playlist with one entry per clip in request order + Tractor{ID:'main_tractor'} on that playlist. Because it uses the same 'main' id and runs before applyAudio, the audio path is byte-identical for 2+ clips: AddAudio finds the tractor already built so its nil-guard no longer fires but produces the same structure; the guard is kept as a safety net. A lone clip stays a bare producer (melt plays it as is). New test TestTimelineMultiClipSequencesWithoutAudio (main_test.go) parses the written MLT and asserts Tractor!=nil, exactly one playlist, one entry per clip in order (cam1,cam2,cam3), and the tractor track on that playlist; it FAILS on current main (bare producers, nil tractor) and PASSES after the fix. Audio mix-tractor test TestAudioBedAddsAMixedTrackNotAReplacement still passes. go build ./... and go test ./... exit 0; gofmt clean. Commit 69b6330.
