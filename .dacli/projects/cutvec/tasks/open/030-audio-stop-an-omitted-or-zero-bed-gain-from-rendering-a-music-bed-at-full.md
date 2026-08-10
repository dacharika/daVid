---
id: t-01KZPF8MGWF5MVKXAY81J6RRS2
kind: task
created: 2026-08-10T18:34:22Z
created_by: a-go-auditor-bhk1rs
owner: a-go-auditor-bhk1rs
priority: should
---
# audio: stop an omitted or zero bed gain from rendering a music bed at full volume over dialogue
## So that
an agent that lays a music bed without naming a gain gets a bed under the dialogue, not full-volume music drowning the speech — matching the catalog schema's promise that the dialogue stays audible beneath
## Acceptance
- [ ] An audioSpec with gain omitted no longer yields a mix transition whose start/end is "1": it produces either a documented low bed default or a clear validation error, pinned by a new Go test that builds the timeline and asserts the mix transition start/end (no worker/render). Evidence: applyAudio cmd/cutvec/effects.go:226-230 accepts gain==0; AddAudio internal/timeline/timeline.go:97-99 rewrites gain<=0 to 1.0; mix uses start=end=gain timeline.go:124-131; schema catalog.go:521,528 promises a bed under dialogue at ~0.15.
- [ ] An explicit gain of 0 behaves per the chosen design and is covered by a test — a silent bed stays 0 OR 0 is rejected with a stable axerror code plus hint — so applyAudio's accepted range, AddAudio's transform, and the catalog schema wording (catalog.go:528) no longer disagree.
- [ ] Existing audio tests still pass (TestAudioBedAddsAMixedTrackNotAReplacement, TestAudioGainOutOfRangeIsRejected, TestAddAudio*); go build ./... and go test ./... both exit 0; gofmt clean. Out of scope: MLT mix rendering, the grade/denoise/sfx effect presets, and the hook/caption overlap (open task 029).
## Log
