---
id: f-omitted-zero-bed-gain-fixed-omitted-0-15-default-explicit-0-rejected-no-more
kind: note
note_kind: finding
created: 2026-08-10T21:13:07Z
created_by: a-fixer-w108mq
about: "[[030]]"
severity: major
---
# Omitted/zero bed gain fixed: omitted->0.15 default, explicit 0 rejected, no more full-volume bed
Root cause: applyAudio (cmd/cutvec/effects.go:226) accepted gain==0 and AddAudio (internal/timeline/timeline.go:97) rewrote gain<=0 to 1.0, so an omitted/zero gain laid a music bed at full scale over dialogue — opposite of the catalog promise (internal/catalog/catalog.go:521,528). Fix: (1) audioSpec.Gain is now *float64 so an omitted JSON key (nil) is distinguished from an explicit 0; nil defaults to timeline.DefaultBedGain=0.15, an explicit value must be in (0,1] else axerror input.invalid + hint. (2) AddAudio's gain<=0 fallback now yields DefaultBedGain, not 1, removing the full-volume footgun for every caller. (3) catalog.go:528 wording updated to 'level in (0,1]; omit for a ~0.15 bed; 0 rejected'. Pins: TestAudioOmittedGainDefaultsToABedUnderDialogue asserts mix start/end==0.15 (not 1); TestAudioExplicitZeroGainIsRejected asserts input.invalid+hint. go build ./... and go test ./... exit 0, gofmt clean. Commit 1df16fa.
