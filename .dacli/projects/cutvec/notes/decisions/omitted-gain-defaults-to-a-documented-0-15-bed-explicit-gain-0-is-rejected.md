---
id: d-omitted-gain-defaults-to-a-documented-0-15-bed-explicit-gain-0-is-rejected
kind: note
note_kind: decision
created: 2026-08-10T21:12:32Z
created_by: a-fixer-w108mq
about: "[[030]]"
---
# Omitted gain defaults to a documented 0.15 bed; explicit gain 0 is rejected
## Chose
Omitted gain defaults to a documented 0.15 bed; explicit gain 0 is rejected
## Rejected
Treat explicit 0 as a valid silent bed (start=end=0)
## Because
The 'so that' narrative wants an omitted gain to WORK as a quiet bed, so omitted->timeline.DefaultBedGain=0.15 (via a *float64 to tell nil-omitted from explicit-0). An explicit 0 is pointless (silence) and was the exact value the old AddAudio rewrote to full scale, so applyAudio rejects it with axerror input.invalid + a hint pointing at the omit-for-default path; AddAudio's gain<=0 fallback now yields DefaultBedGain not 1, killing the full-volume footgun for every caller. A silent-0 bed would keep a meaningless code path and still disagree with the catalog wording.
