---
id: f-reframe-edit-s-punchin-crop-is-a-no-op-on-the-render-melt-uses-use-profile
kind: note
note_kind: finding
created: 2026-08-10T17:33:11Z
created_by: a-go-auditor-vsb4em
about: "[[006]]"
severity: major
---
# reframe-edit's PunchIn crop is a no-op on the render melt (uses use_profile edges 023 proved don't crop)
internal/timeline/timeline.go:267-288 PunchIn builds a 'crop' filter with use_profile=1 and profile-pixel left/right/top/bottom edges. Task 023's VERIFIED decision (d-shorts-head-and-shoulders...) and memory cutvec-melt-crop-fill established against the production melt (melt <tl> -consumer avformat) that 'crop with use_profile edges is a no-op' and only 'crop' with explicit left/right/top/bottom in SOURCE pixels crops-and-fills. The shorts path was migrated to CropWindow (timeline.go:229, source-pixel edges, use_profile deliberately absent — timeline_test.go:205-207 asserts its absence), but reframe-edit was left on PunchIn. Effect: reframe.go:153 applies PunchIn to every shot, so 'reframe-edit' (the recipe whose whole purpose is making a locked-off wide shot look multi-camera) renders NO actual punch-ins — every 'left'/'right' single renders as the untouched wide frame. The command's headline capability is silently broken on the same melt build shorts renders on. Compounding: timeline_test.go:179 asserts use_profile=='1', enshrining the broken form; reframe_test.go only tests PunchIn's clamp math, never that it crops on the real melt. Fix mirrors 023: express PunchIn as source-pixel crop edges (needs src dims, now available via sourceDims/022 probe) or route reframe through CropWindow. Separately, CropRect (timeline.go:216, qtcrop — renders BLACK per the note) is dead code, uncalled in production.
