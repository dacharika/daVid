---
id: d-filed-the-audio-bed-gain-0-becomes-full-volume-defect-as-the-single-highest
kind: note
note_kind: decision
created: 2026-08-10T18:33:46Z
created_by: a-go-auditor-bhk1rs
about: "[[006]]"
---
# Filed the audio-bed gain-0-becomes-full-volume defect as the single highest-value evidence-based change
## Chose
Filed the audio-bed gain-0-becomes-full-volume defect as the single highest-value evidence-based change
## Rejected
Re-filing the reframe-edit PunchIn no-op finding (refuted by VERIFIED decision d-keep-punchin), re-filing the hook/caption SRT overlap (already queued as open task 029), or filing more CropRect/timeline/doctor/progress work (all already done as 022-028)
## Because
The gain contradiction is structural and provable from the source with no melt render: applyAudio (effects.go:226) accepts gain==0 as valid, AddAudio (timeline.go:97) rewrites gain<=0 to 1.0, and the mix transition sets start=end=gain (timeline.go:124), so an omitted or zero gain lays a music bed at FULL scale over the dialogue — the exact opposite of the catalog schema's promise that 'the dialogue stays audible beneath them' (catalog.go:521,528). It is unit-testable in Go by asserting the mix transition's start/end, so no worker or render is needed. It hits a headline capability (the SFX/music-bed half of the podcast edit) at the most common call shape — an agent adding a bed without specifying gain. It is distinct from every open/done task: 029 is the hook/caption SRT overlap, 020 was the grade/denoise/sfx effect PRESETS (picture-filter chains), not the audio-bed gain default. The PunchIn alternative is refuted-adjacent and the crop/timeline/doctor/progress alternatives are already shipped, making this the one distinct, grounded, render-free improvement.
