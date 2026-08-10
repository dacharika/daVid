---
id: d-filed-the-hook-caption-srt-overlap-fix-as-the-single-highest-value-evidence
kind: note
note_kind: decision
created: 2026-08-10T18:08:21Z
created_by: a-go-auditor-zd1s8t
about: "[[006]]"
---
# Filed the hook/caption SRT-overlap fix as the single highest-value evidence-based change
## Chose
Filed the hook/caption SRT-overlap fix as the single highest-value evidence-based change
## Rejected
Re-filing the reframe-edit PunchIn no-op finding (contradicts the VERIFIED decision d-keep-punchin's A/B and would re-file refuted 027 scope), or filing more CropRect/timeline work (already queued as 028/026)
## Because
The overlap is structural and provable from the source without a melt render (hook cue is a fixed [0,2.5s] card at shorts.go:512-513; first caption cue is timed from group[0].Start-start at shorts.go:526 with no clearance), so a fix is unit-testable in Go — this answers the earlier sibling objection that the hook/caption overlap 'cannot be verified without a real melt render.' It hits a headline capability (vertical shorts with hook + karaoke captions) at the highest-value moment (the opening 2.5s retention window). The existing test TestBuildSRTLeadsWithTheHook already builds the overlapping case yet asserts nothing about it, so the defect is latent and green. The reframe PunchIn alternative is refuted-adjacent and CropRect/edit scope is already queued, making this the one distinct, grounded, verifiable improvement.
