---
id: f-shorts-buildsrt-emits-a-hook-cue-that-overlaps-the-first-caption-cue
kind: note
note_kind: finding
created: 2026-08-10T18:08:12Z
created_by: a-go-auditor-zd1s8t
about: "[[006]]"
severity: major
---
# shorts buildSRT emits a hook cue that overlaps the first caption cue
cmd/cutvec/shorts.go:510-514 writes the hook as SRT cue #1 spanning srtTime(0)-->srtTime(2.5) (a hard-coded 2.5s card). The following caption cues are timed independently from the words' own positions: cue #2 starts at srtTime(group[0].Start-start) (shorts.go:525-526). Nothing shifts caption timing to clear the hook, so whenever the first spoken word begins within 2.5s of the clip start — the normal case, since shorts are cut to begin at speech — cue #2 starts before the hook cue ends and the two SRT cues overlap. Overlapping cues are malformed SRT independent of renderer; with MLT's subtitle/libass they render stacked, so the hook card and the first karaoke captions display simultaneously during the opening 2.5s — the exact moment the hook is meant to stand alone. Evidence that this is structural, not hypothetical: the existing test TestBuildSRTLeadsWithTheHook (shorts_test.go:61-67) constructs precisely this case — hook [0,2.5] + a word at [0,1] — yet only asserts the hook is the opening cue and never checks for overlap, so the defect ships green. Unlike a render-dependent question, the overlap is provable in Go by parsing buildSRT's output and comparing cue spans, so a fix is unit-testable without melt. Distinct from open tasks 026 (timeline edit) and 028 (CropRect removal).
