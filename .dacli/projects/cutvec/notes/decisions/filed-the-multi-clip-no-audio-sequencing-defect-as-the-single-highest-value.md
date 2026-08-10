---
id: d-filed-the-multi-clip-no-audio-sequencing-defect-as-the-single-highest-value
kind: note
note_kind: decision
created: 2026-08-10T18:47:15Z
created_by: a-go-auditor-emr68x
about: "[[006]]"
---
# Filed the multi-clip-no-audio sequencing defect as the single highest-value evidence-based change
## Chose
Filed the multi-clip-no-audio sequencing defect as the single highest-value evidence-based change
## Rejected
Re-filing the hook/caption SRT overlap (already open as task 029) or the audio-bed gain-0 default (already open as task 030); re-filing the reframe PunchIn no-op (refuted by verified decision d-keep-punchin); or the applyAudio map-iteration determinism nit (needs 2+ distinct producer targets on audio effects — rare, and hard to pin in a deterministic unit test)
## Because
timelineBuildCmd (main.go:229-256) never calls tl.Sequence(); sequencing only happens as a side effect of AddAudio (timeline.go:92-96), so a 2+ clip 'timeline' build with NO audio writes bare <producer>s with no <playlist>/<tractor> and Validate() (timeline.go:379-404) does not catch it. It is render-free and unit-testable in Go (assert tl.Tractor!=nil and one playlist entry per clip), hits the core 'clips play as one edit' promise on the most basic call shape, and the existing TestTimelineSlice (main_test.go:301-342) builds exactly this 2-clip no-audio case yet asserts nothing about sequencing so the defect ships green. It is distinct from every open task (029 hook overlap, 030 audio gain) and every done task (reframe/shorts/crop/timeline-edit all call Sequence explicitly).
