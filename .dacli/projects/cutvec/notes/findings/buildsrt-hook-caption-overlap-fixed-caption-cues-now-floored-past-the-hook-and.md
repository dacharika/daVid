---
id: f-buildsrt-hook-caption-overlap-fixed-caption-cues-now-floored-past-the-hook-and
kind: note
note_kind: finding
created: 2026-08-11T09:08:31Z
created_by: a-fixer-aj37ya
about: "[[029]]"
severity: major
---
# buildSRT hook/caption overlap fixed: caption cues now floored past the hook and each prior cue's end
Root cause: shorts.go buildSRT wrote the hook as cue [0,2.5] but timed caption cues independently from group[0].Start-start, so any first word spoken inside 2.5s (the normal case) produced an overlapping cue. Fix (cmd/cutvec/shorts.go): added consts hookSeconds=2.5 and minCaptionShow=0.5; buildSRT now tracks a running 'floor' initialized to hookSeconds when a hook is present (0 otherwise) and, per cue, clamps cueStart up to floor, ensures cueEnd>=cueStart+minCaptionShow, then advances floor=cueEnd. This yields strictly non-overlapping cues whether or not a hook is present. AC1: new Go tests parse the SRT and compare consecutive spans — TestBuildSRTHookDoesNotOverlapFirstCaption (first word at t=0, inside the hook) and TestBuildSRTCuesNeverOverlapWithoutHook, plus assertNoCueOverlap/parseCueSpans helpers. AC2: TestBuildSRTLeadsWithTheHook still passes and now also asserts non-overlap. AC3 gates all green: go build ./..., go vet ./..., go test -race ./... (all ok), gofmt -w on touched files, staticcheck exit 0.
