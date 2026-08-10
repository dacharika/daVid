---
id: d-filed-the-timeline-build-missing-validate-gap-build-writes-an-unvalidated
kind: note
note_kind: decision
created: 2026-08-10T21:34:41Z
created_by: a-go-auditor-p03sgd
about: "[[006]]"
---
# Filed the timeline-build missing-Validate gap (build writes an unvalidated timeline) as the single highest-value evidence-based change
## Chose
Filed the timeline-build missing-Validate gap (build writes an unvalidated timeline) as the single highest-value evidence-based change
## Rejected
Re-filing any open task 032-037 (blank element, render atomic publish, shorts rank, assets rank, Clock :60.000 rounding, MCP assets routing) — all near-dups dacli refuses (exit 3); or the applyAudio/speakerAt map-iteration tie-break nit rejected repeatedly (rare, needs an exact tie, hard to pin deterministically)
## Because
Every determinism/audio/render/mcp finding in the brief is now an open task (032-037), so re-filing any is a near-dup. The build-path missing-Validate gap is DISTINCT from all of them and from done 029/030/031, and PROVEN from source with no melt/worker: timelineBuildCmd (main.go:199-286) marshals+writes with no tl.Validate(), while reframe.go:157, shorts.go:242 and timeline_edit.go:231 all validate before writing. It hits the tool's own contract (a written timeline is one melt can render) on the PRIMARY compose surface: two clips with the same explicit id write duplicate <producer id> and return success, deferring the failure to an opaque melt error at render instead of a clean timeline.invalid. Timeline.Validate() already detects the fault (timeline.go:399), so the fix is one guarded call mirroring the sibling paths. Unit-testable in pure Go; main_test.go asserts nothing about duplicate ids or build validation today, so a fix ships green.
