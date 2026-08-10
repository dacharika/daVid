---
id: f-timeline-build-writes-the-mlt-without-ever-calling-tl-validate-so-a-duplicate
kind: note
note_kind: finding
created: 2026-08-10T21:34:31Z
created_by: a-go-auditor-p03sgd
about: "[[006]]"
severity: major
---
# timeline build writes the MLT without ever calling tl.Validate(), so a duplicate producer id (or any structural fault) is written and reported as success instead of failing cleanly
timelineBuildCmd (cmd/cutvec/main.go:199-286) builds the timeline, applies effects, sequences when len(Producers)>=2, applies audio, then goes straight to tl.Marshal() + os.WriteFile (main.go:275-284) with NO tl.Validate() call. EVERY other timeline-producing path validates before writing: reframe.go:157, shorts.go:242, and timeline edit's whole contract is 'a mutated timeline is still one melt can render' (timeline_edit.go:231). The build path — the PRIMARY user-visible surface for composing a timeline — is the one exception. Concrete failure, no melt/worker needed: a build request with two clips carrying the same explicit id, e.g. {"project":"p","clips":[{"id":"dup","resource":"a.mp4"},{"id":"dup","resource":"b.mp4"}]}, yields tl.Producers with duplicate ids; Sequence('main') references both; Marshal writes two <producer id="dup"> and the code returns success (timelineResult.Producers=2). Timeline.Validate() ALREADY detects this ('duplicate producer id %q', timeline.go:399) and 'producer has no resource'/'no producers'/'filter has no mlt_service' — the build path just never asks. Result: a broken timeline lands on disk, render later fails at melt with an opaque error instead of cutvec returning a clean timeline.invalid naming the duplicate. LATENT/GREEN: main_test.go has NO duplicate-id or build-validation assertion (line 1515 covers empty resource only); a test driving timelineBuildCmd with two id=dup clips and asserting code timeline.invalid FAILS today (returns success) and PASSES after adding Validate. DISTINCT from open 032-037 (blank element, render atomic, shorts rank, assets rank, clock rounding, mcp assets routing) and done 029/030/031 (hook overlap, audio gain, sequencing) — 031 added the Sequence call but not validation. go build ./... is green on main.
