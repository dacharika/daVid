---
id: f-croprect-removed-zero-callers-and-cropwindow-test-docs-corrected-to-match-027-s
kind: note
note_kind: finding
created: 2026-08-10T18:12:18Z
created_by: a-fixer-b774vg
about: "[[028]]"
severity: minor
---
# CropRect removed (zero callers) and CropWindow/test docs corrected to match 027's verified use_profile finding
Removed the unused CropRect (formerly internal/timeline/timeline.go:214-218): grep 'CropRect' across cmd/ and internal/ returned only its own definition — zero callers — and it built a qtcrop filter that renders black on the render worker's melt (memory cutvec-melt-crop-fill). Reworded CropWindow's doc comment (timeline.go): it no longer claims the use_profile edge form is a no-op/black; it now states use_profile is a valid crop-and-fill form (PunchIn uses it) and CropWindow omits it purely as a coordinate-space choice (insets already in source pixels), naming the real hazard as center=1 combined with explicit edges. Also corrected the parallel misleading comment/error in timeline_test.go:205-207. Gates: go build ./..., go vet ./..., go test ./... all exit 0; gofmt clean; staticcheck exit 0.
