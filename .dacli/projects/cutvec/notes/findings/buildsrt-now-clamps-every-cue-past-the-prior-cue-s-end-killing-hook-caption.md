---
id: f-buildsrt-now-clamps-every-cue-past-the-prior-cue-s-end-killing-hook-caption
kind: note
note_kind: finding
created: 2026-08-10T21:02:21Z
created_by: a-fixer-24tkme
about: "[[029]]"
severity: major
---
# buildSRT now clamps every cue past the prior cue's end, killing hook/caption overlap
Root cause: buildSRT (cmd/cutvec/shorts.go:507) emitted a fixed hook cue [0,2.5] then timed the first caption cue from group[0].Start-start with no clearance, so a spoken word beginning before 2.5s produced a caption span overlapping the hook card. Fix: added a prevEnd tracker in an emit() closure (shorts.go) that clamps each cue's start to >= the previous cue's end (and end to >= start), applied to the hook and every caption cue uniformly, plus a hookSeconds=2.5 constant. This guarantees no two cues' [start,end) spans overlap, hook present or absent. Tests: new parseSRTCues/parseSRTTime/assertNoCueOverlap helpers in shorts_test.go; TestBuildSRTHookNeverOverlapsFirstCaption (first word at 0.5s, inside the hook) asserts non-overlap and first caption start>=2.5; TestBuildSRTCaptionsDoNotOverlapWithoutHook covers the hook-absent case; TestBuildSRTLeadsWithTheHook updated to also assert non-overlap. go build ./..., go test -race ./..., go vet, staticcheck, gofmt all clean.
