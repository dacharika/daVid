---
id: f-croprect-is-dead-code-and-cropwindow-s-doc-comment-contradicts-027-s-verified
kind: note
note_kind: finding
created: 2026-08-10T17:54:16Z
created_by: a-go-auditor-1mtdcs
about: "[[006]]"
severity: minor
---
# CropRect is dead code and CropWindow's doc comment contradicts 027's verified use_profile finding
Two evidence-based issues in internal/timeline/timeline.go, both left behind by task 027's refutation:

1) CropRect (timeline.go:214-218) has ZERO callers — grep 'CropRect' across cmd/ and internal/ returns only its own definition/comment. It builds a 'qtcrop' filter, which renders BLACK on the render worker's melt (memory cutvec-melt-crop-fill; task 023/027 verification). 027's closing note explicitly said this 'is filed as its own task rather than carried by a refuted one' — but no such task was ever filed (open backlog is only 006/025/026).

2) CropWindow's doc comment (timeline.go:226-228) states the 'use_profile edge form ... render[s] black or no-op'. Task 027's A/B on the real 'melt <tl> -consumer avformat' path REFUTED this: crop with use_profile=1 + explicit edges (no center=1) visibly crops-and-fills, and PunchIn (timeline.go:267-288) relies on exactly this form in production for reframe-edit (reframe.go). The comment enshrines the precise 'wrong belief' 027 warned against; an agent trusting it could delete working PunchIn code and re-break reframe-edit's headline capability.
