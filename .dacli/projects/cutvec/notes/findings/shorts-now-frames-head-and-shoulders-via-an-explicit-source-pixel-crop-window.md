---
id: f-shorts-now-frames-head-and-shoulders-via-an-explicit-source-pixel-crop-window
kind: note
note_kind: finding
created: 2026-08-10T17:13:04Z
created_by: a-fixer-pwpjwm
about: "[[023]]"
severity: major
---
# shorts now frames head-and-shoulders via an explicit source-pixel crop window, verified from rendered frames
buildShortTimeline (cmd/cutvec/shorts.go) no longer emits crop center=1/center_bias. It computes headShouldersWindow(srcW,srcH,cx,cy): a 9:16 window spanning ~0.7*srcH (zoom past full-body), centred on the seat CX, lifted so the face CY sits ~0.34 down (headroom above, shoulders below), clamped inside the source. Source dims come from src.Probe (sourceDims fallback 1920x1080 when unprobed); CY from new seatCY (mirrors seatCentre). The window renders as timeline.CropWindow -> mlt 'crop' filter with left/right/top/bottom source-pixel insets (NOT qtcrop, NOT use_profile — both render black/no-op on the render worker's melt; see decision note). AC1: explicit window, not center=1 — pinned by TestBuildShortTimelineIsVerticalTrimmedAndFiltered (asserts no name=center/center_bias, explicit insets present), TestHeadShouldersWindowFramesHeadAndShoulders/ClampsToEdges, TestCropWindowExpressesTheKeptRectangleAsInsets, TestShortTimelineWindowTracksTheSeatColumn. AC2: verified from an EXTRACTED FRAME, not XML — rendered the exact computed timeline (crop insets left191 right805 top49 bottom167 + subtitle) through the same 'melt <tl> -consumer avformat' the worker uses on a synthetic seated source; the frame fills 1080x1920 with head upper-third + shoulders below and the non-speaking seat removed. Full-body A/B on a head+torso+legs source: center=1 shows head-to-legs; the computed window cuts the legs and frames upper body. Gates: go build, go vet, go test -race ./... all ok; gofmt clean; staticcheck exit 0.
